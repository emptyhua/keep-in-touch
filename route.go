package kit

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	typeOfBytes   = reflect.TypeOf(([]byte)(nil))
	typeOfSession = reflect.TypeOf(&Session{})
)

type Handler struct {
	Receiver reflect.Value  // receiver of method
	Method   reflect.Method // method stub
	Type     reflect.Type   // low-level type of method
	IsRawArg bool           // whether the data need to serialize
}

type Route struct {
	rules map[string]*Handler
}

func NewRoute() *Route {
	return &Route{
		rules: make(map[string]*Handler),
	}
}

func isExported(name string) bool {
	w, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(w)
}

func isHandlerMethod(method reflect.Method) bool {
	mt := method.Type
	// Method must be exported.
	if method.PkgPath != "" {
		return false
	}

	// Method needs three ins: receiver, *Session, []byte or pointer.
	if mt.NumIn() != 3 {
		return false
	}

	if t1 := mt.In(1); t1.Kind() != reflect.Ptr || t1 != typeOfSession {
		return false
	}

	if mt.In(2).Kind() != reflect.Ptr && mt.In(2) != typeOfBytes {
		return false
	}
	return true
}

func (r *Route) Reg(prefix string, service interface{}) {
	serviceValue := reflect.ValueOf(service)
	serviceType := reflect.TypeOf(service)
	serviceTypeName := reflect.Indirect(serviceValue).Type().Name()

	if !isExported(serviceTypeName) {
		panic(errors.New("type " + serviceTypeName + " is not exported"))
	}

	for m := 0; m < serviceType.NumMethod(); m++ {
		method := serviceType.Method(m)
		mt := method.Type
		mn := method.Name
		if isHandlerMethod(method) {
			raw := false
			if mt.In(2) == typeOfBytes {
				raw = true
			}

			if prefix != "" {
				mn = prefix + "." + mn
			}

			mn = strings.ToLower(mn)
			if _, ok := r.rules[mn]; ok {
				panic(fmt.Errorf("route rule with name %s already existed", mn))
			}

			Logger.Infof("route register %s", mn)

			r.rules[mn] = &Handler{
				Receiver: serviceValue,
				Method:   method,
				Type:     mt.In(2),
				IsRawArg: raw,
			}
		}
	}
}

func (r *Route) Exec(s *Session, msg *Message) {
	handler, ok := r.rules[msg.Route]
	if !ok {
		Logger.Errorf("unhandled route %s", msg.Route)
		return
	}

	var payload = msg.Data
	var data interface{}

	if handler.IsRawArg {
		data = payload
	} else {
		data = reflect.New(handler.Type.Elem()).Interface()
		err := json.Unmarshal(payload, data)
		if err != nil {
			Logger.Errorf("json.Unmarshal error %v %v", err, data)
			return
		}

		if req, ok := data.(RequestHeader); ok {
			req.SetMsgId(msg.ID)
		}
	}

	args := []reflect.Value{handler.Receiver, reflect.ValueOf(s), reflect.ValueOf(data)}
	handler.Method.Func.Call(args)
}
