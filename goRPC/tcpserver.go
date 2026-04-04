package tcp

import (
	"net"
	"red-shinku/mygoRPC/handler"
	"red-shinku/mygoRPC/internal/pool"
	"red-shinku/mygoRPC/internal/ports"
	"time"
)

type TcpServer struct {
	listener *net.TCPListener
}

func NewTcpServer() *TcpServer {
	return &TcpServer{}
}

func (s *TcpServer) Init(ipaddr *net.IPAddr, port int) error {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: ipaddr.IP, Port: port})
	if err != nil {
		return err
	}
	s.listener = listener
	return nil
}

func (s *TcpServer) Run() {
	go s.accept()
}

func (s *TcpServer) accept() {
	for {
		conn, err := s.listener.AcceptTCP()
		if err != nil {
			//TODO: log error
			continue
		}
		newconnect := NewConnect(5, conn)
		go newconnect.Work()
	}
}

// for tcp server
type Connect struct {
	conn    *net.TCPConn
	handler ports.Handler
	rqpool  *requestPool
}

func NewConnect(capacity int, conn *net.TCPConn) *Connect {
	c := new(Connect)
	c.conn = conn
	c.handler = handler.NewMyrpcHandler()
	c.rqpool = NewRequestPool(capacity)
	return c
}

// Work : the server goroutinue func read/write loop
func (c *Connect) Work() {
	defer c.conn.Close()
	if err := c.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		return
	}
	c.rqpool.Start(c)
	for {
		headerlen, bodylen, data := c.handler.RecvData(c.conn)
		//将初步解析的包加入协程池任务队列，并将结果自动有序保存
		c.rqpool.HandleData(c.handler, headerlen, bodylen, data)
	}
}

// a small goroutinue pool for a connection
type requestPool struct {
	//协程池, 分析请求并调用函数，结果传到管道
	grpool pool.GoroutinuePool
	result chan []byte
}

func NewRequestPool(capacity int) *requestPool {
	return &requestPool{grpool: pool.NewGoroutinuePool(capacity), result: make(chan []byte, capacity)}
}

// Start : run goroutinuepool(for handle task) and a SendBack goroutinue
func (p *requestPool) Start(cn *Connect) {
	p.grpool.Start()
	//另一个独立协程从管道获取包并发送
	go func() {
		backdata := <-p.result
		cn.handler.SendBack(cn.conn, backdata)
	}()
}

func (p *requestPool) HandleData(h ports.Handler, headerlen uint32, bodylen uint32, data []byte) {
	p.grpool.PutTask(func() {
		result := h.Handle(headerlen, bodylen, data)
		p.result <- result
	})
}
