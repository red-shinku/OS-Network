package goRPC

import (
	"red-shinku/mygoRPC/internal/ports"
	servermain "red-shinku/mygoRPC/internal/server"
	"sync"
)

func CreateServer(server ports.Server, network, address string, port int) {
	once.Do(func() {
		sv = servermain.NewRPCServer(server, network, address, port)
	})
}

var (
	sv   *servermain.RPCServer
	once sync.Once
)

func Registry(name string, f any) {
	sv.Register(name, f)
}
