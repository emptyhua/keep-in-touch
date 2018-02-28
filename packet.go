// Copyright (c) nano Author. All Rights Reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package kit

import (
	"bytes"
	"errors"
)

// Codec constants.
const (
	PacketHeadLength = 4
	PacketMaxSize    = 64 * 1024
)

type PacketType byte

const (
	_ PacketType = iota
	// Handshake represents a handshake: request(client) <====> handshake response(server)
	PacketHandshake = 0x01

	// HandshakeAck represents a handshake ack from client to server
	PacketHandshakeAck = 0x02

	// Heartbeat represents a heartbeat
	PacketHeartbeat = 0x03

	// Data represents a common data packet
	PacketData = 0x04

	// Kick represents a kick off packet
	PacketClose = 0x05 // disconnect message from server
)

// ErrWrongPacketType represents a wrong packet type.
var ErrWrongPacketType = errors.New("kit:wrong packet type")

// ErrPacketSizeExcced is the error used for encode/decode.
var ErrPacketSizeExcced = errors.New("kit:packet size exceed")

// Packet represents a network packet.
type Packet struct {
	Type PacketType
	Data []byte
}

// A PacketDecoder reads and decodes network data slice
type PacketDecoder struct {
	buf  *bytes.Buffer
	size int  // last packet length
	typ  byte // last packet type
}

// NewPacketDecoder returns a new decoder that used for decode network bytes slice.
func NewPacketDecoder() *PacketDecoder {
	return &PacketDecoder{
		buf:  bytes.NewBuffer(nil),
		size: -1,
	}
}

func (c *PacketDecoder) forward() error {
	header := c.buf.Next(PacketHeadLength)
	c.typ = header[0]
	if c.typ < PacketHandshake || c.typ > PacketClose {
		return ErrWrongPacketType
	}
	c.size = bytesToInt(header[1:])

	// packet length limitation
	if c.size > PacketMaxSize {
		return ErrPacketSizeExcced
	}
	return nil
}

// Decode decode the network bytes slice to packet.Packet(s)
// TODO(Warning): shared slice
func (c *PacketDecoder) Decode(data []byte) ([]*Packet, error) {
	c.buf.Write(data)

	var (
		packets []*Packet
		err     error
	)

	// check length
	if c.buf.Len() < PacketHeadLength {
		return nil, nil
	}

	// first time
	if c.size < 0 {
		if err = c.forward(); err != nil {
			return nil, err
		}
	}

	for c.size <= c.buf.Len() {
		p := &Packet{
			Type: PacketType(c.typ),
			Data: make([]byte, c.size),
		}
		copy(p.Data, c.buf.Next(c.size))
		packets = append(packets, p)

		// more packet
		if c.buf.Len() < PacketHeadLength {
			c.size = -1
			break
		}

		if err = c.forward(); err != nil {
			return nil, err

		}
	}

	return packets, nil
}

// Encode create a packet.Packet from  the raw bytes slice and then encode to network bytes slice
// Protocol refs: https://github.com/NetEase/pomelo/wiki/Communication-Protocol
//
// -<type>-|--------<length>--------|-<data>-
// --------|------------------------|--------
// 1 byte packet type, 3 bytes packet data length(big end), and data segment
func (p *Packet) Encode() ([]byte, error) {
	if p.Type < PacketHandshake || p.Type > PacketClose {
		return nil, ErrWrongPacketType
	}

	buf := make([]byte, len(p.Data)+PacketHeadLength)
	buf[0] = byte(p.Type)

	copy(buf[1:PacketHeadLength], intToBytes(len(p.Data)))
	copy(buf[PacketHeadLength:], p.Data)

	return buf, nil
}

// Decode packet data length byte to int(Big end)
func bytesToInt(b []byte) int {
	result := 0
	for _, v := range b {
		result = result<<8 + int(v)
	}
	return result
}

// Encode packet data length to bytes(Big end)
func intToBytes(n int) []byte {
	buf := make([]byte, 3)
	buf[0] = byte((n >> 16) & 0xFF)
	buf[1] = byte((n >> 8) & 0xFF)
	buf[2] = byte(n & 0xFF)
	return buf
}
