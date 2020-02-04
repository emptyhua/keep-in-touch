package kit

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	SessionStatusNormal = iota
	SessionStatusClosed
)

var SessionMaxDelayMsgCount = 100 // 必须比 KitConnWriteQueueSize 小

type SessionCloseEventListener interface {
	OnSessionClose(s *Session)
}

type Session struct {
	sync.RWMutex
	Manager        *SessionManager
	Id             string
	LostConnection time.Time
	status         int
	conn           *KitConn
	data           map[string]interface{}
	delayMsgs      []*Message
}

func newSession(m *SessionManager) *Session {
	return &Session{
		Manager: m,
		Id:      uuid.New().String(),
		status:  SessionStatusNormal,
		data:    make(map[string]interface{}),
	}
}

func (s *Session) String() string {
	return fmt.Sprintf("Session(%s, conn=%v)", s.Id[0:8], s.conn)
}

func (s *Session) Close(reason string) {
	if s.status == SessionStatusClosed {
		return
	}
	s.status = SessionStatusClosed

	Logger.Debugf("%v closed for reason %s", s, reason)

	if s.conn != nil {
		s.conn.Session = nil
		s.conn.Close("session closed") // force close
		s.conn = nil
	}

	for _, v := range s.data {
		if h, ok := v.(SessionCloseEventListener); ok {
			h.OnSessionClose(s)
		}
	}
	s.data = nil

	s.Manager.removeSession(s)
	s.Manager = nil
}

func (s *Session) getConn() *KitConn {
	return s.conn
}

func (s *Session) setConn(conn *KitConn) {
	if s.status != SessionStatusNormal {
		Logger.Warnf("%v.SetConn(%v) status != Normal return", s, conn)
		return
	}

	if s.conn == conn {
		Logger.Warnf("%v.SetConn(%v) old == new return", s, conn)
		return
	}

	if s.conn != nil {
		s.conn.Session = nil
		s.conn.Close("replaced by new connection")
	}

	Logger.Debugf("%v.SetConn(%v)", s, conn)

	s.conn = conn
	s.LostConnection = time.Time{}

	if len(s.delayMsgs) > 0 {
		Logger.Debugf("%v write delayMsgs %d", s, len(s.delayMsgs))
		for _, msg := range s.delayMsgs {
			s.conn.WriteMsg(msg)
		}
		s.delayMsgs = nil
	}
}

func (s *Session) lostConn() {
	if s.conn != nil {
		s.conn.Session = nil
		s.conn = nil
		s.LostConnection = time.Now()
		Logger.Debugf("%v.LostConn()", s)
	}
}

func (s *Session) Push(route string, v interface{}) error {
	return s.Write(MessagePush, 0, route, v)
}

func (s *Session) Response(req RequestHeader, v interface{}) error {
	return s.Write(MessageRequest, req.GetMsgId(), "", v)
}

func (s *Session) Write(t MessageType, msgId uint, route string, data interface{}) error {
	if s.status == SessionStatusClosed {
		return fmt.Errorf("%v write closed session", s)
	}

	msg := NewMessage(t, msgId, route, data)

	if s.conn == nil {
		if len(s.delayMsgs) >= SessionMaxDelayMsgCount {
			return fmt.Errorf("%v delayMsgs reach max count %d", s, SessionMaxDelayMsgCount)
		} else {
			s.delayMsgs = append(s.delayMsgs, msg)
		}
	} else {
		return s.conn.WriteMsg(msg)
	}

	return nil
}

func (s *Session) Set(key string, value interface{}) {
	s.Lock()
	defer s.Unlock()

	s.data[key] = value
}

func (s *Session) HasKey(key string) bool {
	s.RLock()
	defer s.RUnlock()

	_, has := s.data[key]
	return has
}

func (s *Session) Value(key string) interface{} {
	s.RLock()
	defer s.RUnlock()

	return s.data[key]
}
