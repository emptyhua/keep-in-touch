package kit

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	KitConnStatusCreated = iota
	KitConnStatusHandshake
	KitConnStatusWorking
	KitConnStatusClosed
)

const (
	KitConnWriteQueueSize = 128
)

var (
	ErrInvalidConnStatus = errors.New("write conn while not working status")
	ErrBufferExceed      = errors.New("session send buffer exceed")

	kitConnId = uint32(0)

	HeartbeatPacket []byte
	ConnClosePacket []byte
)

type HandshakeHead struct {
	SessionId string `json:"sid"`
}

type KitConn struct {
	Id             uint32
	Server         *Server
	Session        *Session
	conn           net.Conn
	status         int
	mutex          sync.Mutex
	wg             sync.WaitGroup
	decoder        *PacketDecoder
	writeQueue     chan []byte
	cancelRead     chan bool
	heartbeatTimer *time.Ticker
}

func init() {
	HeartbeatPacket, _ = (&Packet{Type: PacketHeartbeat}).Encode()
	ConnClosePacket, _ = (&Packet{Type: PacketClose}).Encode()
}

func NewKitConn(server *Server, conn net.Conn) *KitConn {
	kitConnId++

	kitConn := &KitConn{
		Id:             kitConnId,
		Server:         server,
		conn:           conn,
		status:         KitConnStatusCreated,
		decoder:        NewPacketDecoder(),
		writeQueue:     make(chan []byte, KitConnWriteQueueSize),
		cancelRead:     make(chan bool),
		heartbeatTimer: time.NewTicker(server.HeartbeatInterval),
	}
	return kitConn
}

func (c *KitConn) String() string {
	if c.conn != nil {
		return fmt.Sprintf("KitConn(remote:%v id:%d)", c.conn.RemoteAddr(), c.Id)
	} else {
		return fmt.Sprintf("KitConn(closed id:%d)", c.Id)
	}
}

func (c *KitConn) Close(reason string) {
	c.mutex.Lock()
	if c.status == KitConnStatusClosed {
		c.mutex.Unlock()
		Logger.Warnf("%v.Close(%s) already closed return", c, reason)
		return
	}
	c.status = KitConnStatusClosed
	c.mutex.Unlock()

	Logger.Debugf("%v.Close(%s)", c, reason)

	c.Server = nil

	if c.Session != nil && c.Session.getConn() == c {
		c.Session.lostConn()
	}
	c.Session = nil

	close(c.cancelRead) // 取消读

	if len(c.writeQueue) < KitConnWriteQueueSize {
		c.writeQueue <- ConnClosePacket
	}
}

func (c *KitConn) WriteMsg(msg *Message) error {
	if c.status != KitConnStatusWorking {
		return ErrInvalidConnStatus
	}

	if len(c.writeQueue) >= KitConnWriteQueueSize {
		return ErrBufferExceed
	}

	payload, err := msg.Encode()
	if err != nil {
		panic(err)
	}

	packet := &Packet{Type: PacketData, Data: payload}
	d, err := packet.Encode()
	if err != nil {
		panic(err)
	}

	c.writeQueue <- d
	return nil
}

func (c *KitConn) Handle() {
	c.wg.Add(2)
	go c.writeWorker()
	c.readWorker()
	c.wg.Wait()
	if c.status != KitConnStatusClosed {
		c.Close("read & write existed")
	}
	c.heartbeatTimer.Stop()
	c.conn.Close()
	c.conn = nil
}

func (c *KitConn) writeWorker() {
	defer c.wg.Done()

	for {
		select {
		case <-c.heartbeatTimer.C:
			c.writeQueue <- HeartbeatPacket
		case data := <-c.writeQueue:
			if _, err := c.conn.Write(data); err != nil {
				Logger.Debugf("%v write error: %v", c, err)
				return
			}

			// connection closed and all buf writed
			if c.status == KitConnStatusClosed && len(c.writeQueue) == 0 {
				return
			}
		}
	}
}

func (c *KitConn) readWorker() {
	defer c.wg.Done()

	buf := make([]byte, 2048)
	for {
		select {
		case <-c.cancelRead:
			return
		default:
		}

		n, err := c.conn.Read(buf)
		if err != nil {
			Logger.Debugf("%v read error: %v", c, err)
			return
		}

		packets, err := c.decoder.Decode(buf[:n])
		if err != nil {
			Logger.Errorf("%v decode.Decode error: %v", c, err)
			return
		}

		if len(packets) < 1 {
			continue
		}

		// process all packet
		for _, p := range packets {
			if err := c.processPacket(p); err != nil {
				Logger.Errorf("%v processPacket error %v", c, err)
				return
			}
		}
	}
}

func (c *KitConn) processPacket(p *Packet) error {
	if c.status == KitConnStatusClosed {
		return nil
	}
	switch p.Type {
	case PacketHandshake:
		{
			if c.status != KitConnStatusCreated {
				return fmt.Errorf("%v unexpected handshake from client", c)
			}

			var session *Session = nil

			if len(p.Data) > 0 {
				handInfo := HandshakeHead{}
				if err := json.Unmarshal(p.Data, &handInfo); err != nil {
					return fmt.Errorf("%v invalid handshake data %s", c, string(p.Data))
				}

				if len(handInfo.SessionId) > 0 {
					session = c.Server.SessionManager.GetSessionById(handInfo.SessionId)
				}
			}

			if session == nil {
				session = c.Server.SessionManager.createSession()
				Logger.Debugf("%v create new session %s", c, session.Id)
			} else {
				Logger.Debugf("%v find old session %s", c, session.Id)
			}
			c.Session = session

			data, _ := json.Marshal(map[string]interface{}{
				"code": 200,
				"hb":   c.Server.HeartbeatInterval / time.Second,
				"sid":  session.Id,
			})

			handshakePacket, _ := (&Packet{Type: PacketHandshake, Data: data}).Encode()

			c.writeQueue <- handshakePacket
			c.status = KitConnStatusHandshake
			Logger.Debugf("%v send handshake to client", c)
		}
	case PacketHandshakeAck:
		c.status = KitConnStatusWorking
		Logger.Debugf("%v receiv handshake ack", c)
		if c.Session != nil {
			c.Session.setConn(c)
		}
	case PacketData:
		if c.status < KitConnStatusWorking {
			return fmt.Errorf("%v receiv data before handshake ack", c)
		}

		msg, err := DecodeMessageFromRaw(p.Data)
		if err != nil {
			return err
		}

		Logger.Debugf("%v got msg %v", c, msg)
		c.Server.Route.Exec(c.Session, msg)
	case PacketClose:
		// 客户端主动关闭Session
		Logger.Debugf("%v receiv session close packet", c)
		if c.Session != nil {
			c.Session.Close("closed by client")
		}
	case PacketHeartbeat:
	default:
	}

	return nil
}
