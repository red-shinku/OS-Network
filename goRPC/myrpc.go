package protocal

//this package depend on "registry package"
//It use the information in function-map
//to decode and restore the function args

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"red-shinku/mygoRPC/internal/ports"
	funcregistry "red-shinku/mygoRPC/internal/registry"
	"reflect"
	"sync"
)

type Coding struct {
	registry ports.ServerRegistry
}

var (
	codec ports.LetsCode
	once  sync.Once
)

// TODO: 单例模式
func NewCoding() ports.LetsCode {
	once.Do(func() {
		codec = &Coding{funcregistry.NewServerRegistry()}
	})
	return codec
}

type Header struct {
	Method string
	Err    error
}

func (h *Header) WriteError(err error) {
	h.Err = err
}

func (h *Header) GetError() error {
	return h.Err
}

func (h *Header) WrtieMethod(name string) {
	h.Method = name
}

func (h *Header) GetMethod() string {
	return h.Method
}

type Body struct {
	Args []json.RawMessage `json:"args"`
}

func (b *Body) WriteArgsData(args []json.RawMessage) {
	b.Args = args
}

func (b *Body) ReadArgData() []json.RawMessage {
	return b.Args
}

//TODO: Header/Body添加读写方法接口

type Args = []reflect.Value

func (c *Coding) NewHeader() ports.Header {
	return &Header{}
}

func (c *Coding) NewBody() ports.Body {
	return &Body{}
}

func (codec *Coding) Encode(header ports.Header, body ports.Body) []byte {
	headerValue, ok := header.(*Header)
	if !ok {
		return nil
	}
	headerData, err1 := json.Marshal(headerValue)
	if err1 != nil {
		//TODO: log error
		return nil
	}
	headerlen := len(headerData)

	bodyValue, okb := body.(*Body)
	if !okb {
		return nil
	}
	bodyData, err2 := json.Marshal(bodyValue)
	if err2 != nil {
		//TODO: log error
		return nil
	}
	bodylen := len(bodyData)

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(headerlen))
	binary.Write(buf, binary.BigEndian, uint32(bodylen))
	buf.Write(headerData)
	buf.Write(bodyData)

	return buf.Bytes()
}

func (codec *Coding) Decode(data []byte, headerlen uint32, bodylen uint32) (ports.Header, Args, error) {
	var args Args
	var cur uint32 = 0

	header := Header{}
	if err1 := json.Unmarshal(data[cur:cur+headerlen], &header); err1 != nil {
		//TODO: log error
		return Header{}, Args{}, err1
	}
	cur = cur + headerlen

	body := Body{}
	if err1 := json.Unmarshal(data[cur:cur+bodylen], &body); err1 != nil {
		return Header{}, Args{}, err1
	}

	argsT := codec.registry.GetFuncArgsT(header.Method)
	if argsT == nil {
		return Header{}, Args{}, errors.New("<UNK>")
	}
	for i, typ := range argsT {
		var val reflect.Value
		if typ.Kind() == reflect.Ptr {
			// if we get a ptr type
			// get the true elem type, and make a ptr point to it
			val = reflect.New(typ.Elem())
		} else {
			val = reflect.New(typ)
		}

		var target any
		if val.Kind() == reflect.Ptr {
			target = val.Interface()
		} else {
			target = val.Addr().Interface()
		}

		if err := json.Unmarshal(body.Args[i], target); err != nil {
			// TODO: front log error
			return Header{}, Args{}, err
		}
		args[i] = val
	}
	return header, args, nil
}

//func (codec *Coding) GetProtoLen(reader *bufio.Reader) (uint32, uint32) {
//	lenBuf := make([]byte, 4)
//	if _, err := io.ReadFull(reader, lenBuf); err != nil {
//		//TODO: error logs
//		//how to handle? 考虑协议末尾\n\n并直接清掉
//	}
//	headerlen := uint32(binary.LittleEndian.Uint32(lenBuf))
//
//	if _, err := io.ReadFull(reader, lenBuf); err != nil {
//		//TODO: error logs
//		//how to handle? 考虑协议末尾\n\n并直接清掉
//	}
//	bodylen := uint32(binary.LittleEndian.Uint32(lenBuf))
//	return headerlen, bodylen
//}
