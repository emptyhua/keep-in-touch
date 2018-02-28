package kit

type RequestHeader interface {
	SetMsgId(id uint)
	GetMsgId() uint
}

type RequestHead struct {
	MsgId uint `json:"-"`
}

func (req *RequestHead) SetMsgId(id uint) {
	req.MsgId = id
}

func (req *RequestHead) GetMsgId() uint {
	return req.MsgId
}
