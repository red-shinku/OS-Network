package handler

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"red-shinku/mygoRPC/internal/ports"
	funcregistry "red-shinku/mygoRPC/internal/registry"
	"red-shinku/mygoRPC/protocal"
	"reflect"
	"sync"
)

// MyrpcHandler :Implement interface "Handler"
type MyrpcHandler struct {
	codec   ports.LetsCode
	servReg ports.ServerRegistry
}

var (
	myrpchandler ports.Handler
	once         sync.Once
)

func NewMyrpcHandler() ports.Handler {
	once.Do(func() {
		c := protocal.NewCoding()
		s := funcregistry.NewServerRegistry()
		myrpchandler = &MyrpcHandler{codec: c, servReg: s}
	})
	return myrpchandler
}

// RecvData :get json sequence data which include header and body
func (h *MyrpcHandler) RecvData(conn net.Conn) (uint32, uint32, []byte) {
	reader := bufio.NewReader(conn)
	headerlen, bodylen := h.getProtoLen(reader)
	data := make([]byte, headerlen+bodylen)
	if _, err := io.ReadFull(reader, data); err != nil {
		//TODO: logs error
		return 0, 0, nil
	}
	return headerlen, bodylen, data
}

// getProtoLen :just get lenth of protocol
func (h *MyrpcHandler) getProtoLen(reader *bufio.Reader) (uint32, uint32) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, lenBuf); err != nil {
		//TODO: error logs
		//how to handle? 考虑协议末尾\n\n并直接清掉
	}
	headerlen := uint32(binary.LittleEndian.Uint32(lenBuf))

	if _, err := io.ReadFull(reader, lenBuf); err != nil {
		//TODO: error logs
		//how to handle? 考虑协议末尾\n\n并直接清掉
	}
	bodylen := uint32(binary.LittleEndian.Uint32(lenBuf))
	return headerlen, bodylen
}

// Handle : call & gen response and return
func (h *MyrpcHandler) Handle(headerlen uint32, bodylen uint32, data []byte) []byte {
	header, args, err := h.codec.Decode(data, headerlen, bodylen)
	if err != nil {
		return nil
	}

	result, err := h.callFunc(header.GetValue().Method, args)
	if err != nil {
		header.GetValue().Err = err
		return nil
	}
	backdata, err1 := h.genResponse(header.GetValue().Method, result)
	if err1 != nil {
		header.Err = err1
		return nil
	}
	return backdata
}

// callFunc :find method in server and call
func (h *MyrpcHandler) callFunc(name string, args []reflect.Value) ([]reflect.Value, error) {
	f, err := h.servReg.GetFuncType(name)
	if err != nil {
		//TODO: cannot find method in server
		return nil, err
	}
	ftype := reflect.TypeOf(f)
	if ftype.NumIn() != len(args) {
		//TODO: error logs arg num not match
		return nil, errors.New("args num not match")
	}
	results := f.Call(args)
	return results, nil
}

func (h *MyrpcHandler) genResponse(name string, result []reflect.Value) ([]byte, error) {
	body := h.codec.NewBody()
	for i, rsval := range result {
		data, err := json.Marshal(rsval.Interface())
		if err != nil {
			//TODO: log error
			return nil, err
		}
		body.Args[i] = json.RawMessage(data)
	}
	header := h.codec.NewHeader()
	header.Method = name
	backdata := h.codec.Encode(header, body)
	if backdata == nil {
		//logs error
		return backdata, errors.New("back data Encode failed")
	}
	return backdata, nil
}

func (h *MyrpcHandler) SendBack(conn net.Conn, backdata []byte) error {
	writer := bufio.NewWriter(conn)
	if _, err := writer.Write(backdata); err != nil {
		//TODO: logs error
		return err
	}
	if err := writer.Flush(); err != nil {
		//TODO: logs error
		return err
	}
	return nil
}

//old version
// Handle :decode and call func, then rewrite header, then encode new '[]byte'
//func (h *MyrpcHandler) Handle(conn net.Conn) error {
//	headerlen, bodylen, data := h.getData(conn)
//	if data == nil {
//		return errors.New("Handle failed")
//	}
//	h.handle(conn, headerlen, bodylen, data)
//	return nil
//}
