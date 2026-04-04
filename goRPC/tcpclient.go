package tcp

import (
	"net"
	"red-shinku/mygoRPC/internal/pool"
	"red-shinku/mygoRPC/internal/ports"
)

//TODO : 客户端连接池
//choose连接方法，以实现不同算法

type TcpClient struct {
	conns []*ClientConnect //?
}

// TODO : 客户端连接类，或许应该纯传输，解析的部分在clinetmain内
type ClientConnect struct {
	conn     *net.TCPConn
	cHandler ports.ClientHandler
	sendBuf  chan []byte
	//TODO: 单连接发送并管理复数个请求
	//多协程构造并发送--等结果循环，，协程池
	resultBuf chan []byte
	gpool     pool.GoroutinuePool
}

func NewClientConnPool(conn *net.TCPConn, capacity int) *ClientConnect {
	return &ClientConnect{conn: conn,
		sendBuf:   make(chan []byte, capacity),
		resultBuf: make(chan []byte, capacity),
		gpool:     pool.NewGoroutinuePool(capacity)}
}

func (cc *ClientConnect) work() {
	for {
		senddata := <-cc.sendBuf
		cc.conn.Write(senddata)
		headerlen, bodylen, responsedata := cc.cHandler.RecvData(cc.conn)
		cc.cHandler.HandleResponse(call, responsedata, headerlen, bodylen)
	}
}

func (cc *ClientConnect) run() {
	cc.gpool.Start()
	for i := 0; i < len(cc.sendBuf); i++ {
		cc.gpool.PutTask(cc.work)
	}
}
