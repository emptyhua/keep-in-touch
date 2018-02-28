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
	"encoding/json"
	"errors"
	"fmt"
)

// Type represents the type of message, which could be Request/Notify/Response/Push
type MessageType byte

// Message types
const (
	MessageRequest  MessageType = 0x00
	MessageNotify               = 0x01
	MessageResponse             = 0x02
	MessagePush                 = 0x03
)

const (
	msgTypeMask        = 0x07
	msgRouteLengthMask = 0xFF
	msgHeadLength      = 0x02
)

var messageTypes = map[MessageType]string{
	MessageRequest:  "Request",
	MessageNotify:   "Notify",
	MessageResponse: "Response",
	MessagePush:     "Push",
}

// Errors that could be occurred in message codec
var (
	ErrWrongMessageType = errors.New("wrong message type")
	ErrInvalidMessage   = errors.New("invalid message")
)

// Message represents a unmarshaled message or a message which to be marshaled
type Message struct {
	Type  MessageType // message type
	ID    uint        // unique id, zero while notify mode
	Route string      // route for locating service
	Data  []byte      // payload
}

// String, implementation of fmt.Stringer interface
func (m *Message) String() string {
	return fmt.Sprintf("Type: %s, ID: %d, Route: %s, BodyLength: %d",
		messageTypes[m.Type],
		m.ID,
		m.Route,
		len(m.Data))
}

// Encode marshals message to binary format. Different message types is corresponding to
// different message header, message types is identified by 2-4 bit of flag field. The
// relationship between message types and message header is presented as follows:
// ------------------------------------------
// |   type   |  flag  |       other        |
// |----------|--------|--------------------|
// | request  |----000-|<message id>|<route>|
// | notify   |----001-|<route>             |
// | response |----010-|<message id>        |
// | push     |----011-|<route>             |
// ------------------------------------------
// The figure above indicates that the bit does not affect the type of message.
// See ref: https://github.com/lonnng/nano/blob/master/docs/communication_protocol.md
func (m *Message) Encode() ([]byte, error) {
	if m.Type < MessageRequest || m.Type > MessagePush {
		return nil, ErrWrongMessageType
	}

	buf := make([]byte, 0)
	flag := byte(m.Type) << 1

	buf = append(buf, flag)

	if m.Type == MessageRequest || m.Type == MessageResponse {
		n := m.ID
		// variant length encode
		for {
			b := byte(n % 128)
			n >>= 7
			if n != 0 {
				buf = append(buf, b+128)
			} else {
				buf = append(buf, b)
				break
			}
		}
	}

	if m.Type == MessageRequest || m.Type == MessageNotify || m.Type == MessagePush {
		buf = append(buf, byte(len(m.Route)))
		buf = append(buf, []byte(m.Route)...)
	}

	buf = append(buf, m.Data...)
	return buf, nil
}

// Decode unmarshal the bytes slice to a message
// See ref: https://github.com/lonnng/nano/blob/master/docs/communication_protocol.md
func DecodeMessageFromRaw(data []byte) (*Message, error) {
	if len(data) < msgHeadLength {
		return nil, ErrInvalidMessage
	}

	m := &Message{}
	flag := data[0]
	offset := 1
	m.Type = MessageType((flag >> 1) & msgTypeMask)

	if m.Type < MessageRequest || m.Type > MessagePush {
		return nil, ErrWrongMessageType
	}

	if m.Type == MessageRequest || m.Type == MessageResponse {
		id := uint(0)
		// little end byte order
		// WARNING: must can be stored in 64 bits integer
		// variant length encode
		for i := offset; i < len(data); i++ {
			b := data[i]
			id += uint(b&0x7F) << uint(7*(i-offset))
			if b < 128 {
				offset = i + 1
				break
			}
		}
		m.ID = id
	}

	if m.Type == MessageRequest || m.Type == MessageNotify || m.Type == MessagePush {
		rl := data[offset]
		offset++
		m.Route = string(data[offset:(offset + int(rl))])
		offset += int(rl)
	}

	m.Data = data[offset:]
	return m, nil
}

func serializeOrRaw(v interface{}) ([]byte, error) {
	if data, ok := v.([]byte); ok {
		return data, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func NewMessage(t MessageType, id uint, route string, data interface{}) *Message {
	rawBytes, err := serializeOrRaw(data)
	if err != nil {
		panic(err)
	}

	return &Message{
		Type:  t,
		ID:    id,
		Route: route,
		Data:  rawBytes,
	}
}
