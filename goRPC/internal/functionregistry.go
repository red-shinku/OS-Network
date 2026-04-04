package funcregistry

import (
	"errors"
	"red-shinku/mygoRPC/internal/ports"
	registerclient "red-shinku/mygoRPC/internal/registry/client"
	"reflect"
	"sync"
)

type ServerFAT struct {
	//method name to its args' reflect Type
	funcargs map[string][]reflect.Type
	//method name to its reflection Value
	funcptr map[string]reflect.Value
}

var (
	serverFuncTable ports.ServerRegistry
	once            sync.Once
)

// 单例模式
func NewServerRegistry() ports.ServerRegistry {
	once.Do(func() {
		serverFuncTable = &ServerFAT{}
	})
	return serverFuncTable
}

func (localrs *ServerFAT) GetFuncArgsT(funcname string) []reflect.Type {
	types, ok := localrs.funcargs[funcname]
	if !ok {
		//TODO: log method not found
		return nil
	} else {
		return types
	}
}

func (localrs *ServerFAT) GetFuncType(funcname string) (reflect.Value, error) {
	value, ok := localrs.funcptr[funcname]
	if !ok {
		//TODO: logs error
		return reflect.Value{}, errors.New("Function" + funcname + "Not Found")
	}
	return value, nil
}

func (localrs *ServerFAT) RegisterFunc(name string, f any) error {
	if _, ok := localrs.funcargs[name]; ok {
		//TODO: logs error
		return errors.New("Function" + name + "Already Exist")
	}
	fvalue := reflect.ValueOf(f)
	ftype := fvalue.Type()
	if fvalue.Kind() != reflect.Func {
		//TODO: logs error
		return errors.New("Function " + name + " is not a function")
	}
	argsnum := ftype.NumIn()
	argstype := make([]reflect.Type, argsnum)
	for i := 0; i < argsnum; i++ {
		argstype[i] = ftype.In(i)
	}
	localrs.funcargs[name] = argstype
	return nil
}

// For client, It has a local func-registry to find server
type ClientFuncTab struct {
	// method map to address:port
	function map[string]string
}

var (
	clientFuncTab ports.ClientRegistry
	onceCFT       sync.Once
)

func NewClientRegistry() ports.ClientRegistry {
	onceCFT.Do(func() {
		clientFuncTab = &ClientFuncTab{}
	})
	return clientFuncTab
}

func (ct *ClientFuncTab) SearchFunc(name string) string {
	if addr, ok := ct.function[name]; ok {
		return addr
	}
	return ct.pullFunc(name)
}

// get server function name and server ip address
func (ct *ClientFuncTab) pullFunc(funcname string) string {
	addr := registerclient.FindFunc(funcname)
	if addr == "" {
		//TODO: pullFunc: func not found
		return ""
	}
	ct.function[funcname] = addr
	return addr
}
