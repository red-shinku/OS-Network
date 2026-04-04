package handler

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"red-shinku/mygoRPC/internal/ports"
	"red-shinku/mygoRPC/protocal"
	"reflect"
	"sync"
)

type MyrpcClientHandler struct {
	codec ports.LetsCode
}

var (
	myrpcCH     ports.ClientHandler
	onceMyrpcCH sync.Once
)

func NewMyrpcClientHandler() ports.ClientHandler {
	onceMyrpcCH.Do(func() {
		myrpcCH = &MyrpcClientHandler{codec: protocal.NewCoding()}
	})
	return myrpcCH
}

func (ch *MyrpcClientHandler) GenRequest(name string, args ...any) ([]byte, error) {
	header := ch.codec.NewHeader()
	header.WrtieMethod(name)

	body := ch.codec.NewBody()
	bodydata := make([]json.RawMessage, len(args))
	for i, arg := range args {
		jsondata, err := json.Marshal(arg)
		if err != nil {
			//TODO: logs error
		}
		bodydata[i] = jsondata
	}
	body.WriteArgsData(bodydata)

	request := ch.codec.Encode(header, body)
	if request == nil {
		return nil, errors.New("GenRequest fail in Encode")
	}
	return request, nil
}

func (ch *MyrpcClientHandler) CreateCall() ports.Call {
	return &Call{}
}

// RecvData :get json sequence data which include header and body
func (ch *MyrpcClientHandler) RecvData(conn net.Conn) (uint32, uint32, []byte) {
	reader := bufio.NewReader(conn)
	headerlen, bodylen := ch.getProtoLen(reader)
	data := make([]byte, headerlen+bodylen)
	if _, err := io.ReadFull(reader, data); err != nil {
		//TODO: logs error
		return 0, 0, nil
	}
	return headerlen, bodylen, data
}

// getProtoLen :just get lenth of protocol
func (ch *MyrpcClientHandler) getProtoLen(reader *bufio.Reader) (uint32, uint32) {
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

// TODO: Call 接口?
func (ch *MyrpcClientHandler) HandleResponse(call ports.Call, data []byte, headlen uint32, body uint32) {
	callT := call.(*Call)
	header, args, err := ch.codec.Decode(data, headlen, body)
	if err != nil {
		//TODO: error logs
		callT.Err = err
		callT.done <- callT
		return
	}
	if serverErr := header.GetError(); serverErr != nil {
		callT.Err = serverErr
		callT.done <- callT
		return
	}
	callT.Result = args
	callT.done <- callT
	return
}

// Call : User use it to get result
type Call struct {
	Result []reflect.Value
	Err    error
	done   chan *Call
}

// Done : wait the result
func (c *Call) Done() {
	<-c.done
}

func (c *Call) GetResult() []reflect.Value {
	return c.Result
}
