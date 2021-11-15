package main

import (
	"encoding/binary"
	"net"
	"project/test/connector"
	"sync"

	"github.com/big-larry/suckutils"
)

type listener struct {
	listener    net.Listener
	connections *connections
}

type connections struct {
	connectors map[ServiceName][]*connector.Connector
	rwmux      sync.RWMutex
}

func (c *Configurator) NewListener(network, address string, isLocal bool) (*listener, error) {
	// if network == "unix" {
	// 	if err := os.RemoveAll(address); err != nil {
	// 		return nil, err
	// 	}
	// }
	ln, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	result := &listener{listener: ln, connections: &connections{connectors: make(map[ServiceName][]*connector.Connector)}}
	go result.accept(&connectordata{configurator: c}, isLocal)
	return result, nil
}

func (listener *listener) accept(data *connectordata, isLocal bool) {
	conn, err := listener.listener.Accept()
	if err != nil {
		l.Error("accept", err)
		return
	}

	buf := make([]byte, 4)
	_, err = conn.Read(buf)
	if err != nil {
		return
	}

	buf = make([]byte, binary.BigEndian.Uint32(buf))
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	name := ServiceName(buf[:n])
	if isLocal {
		data.configurator.localservices.rwmux.RLock()
		if _, ok := data.configurator.localservices.serviceinfo[name]; !ok {
			conn.Close()
			l.Warning("locallistener", suckutils.ConcatThree("unknown trying to connect from ", conn.RemoteAddr().String(), ", connection refused"))
			data.configurator.localservices.rwmux.RUnlock()
			return
		}
		data.configurator.localservices.rwmux.RUnlock()

	} else {
		if name != ConfServiceName {
			conn.Close()
			l.Warning("remotelistener", suckutils.ConcatThree("unknown trying to connect from ", conn.RemoteAddr().String(), ", connection refused"))
			return
		}
	}
	data.servicename = name
	if isLocal {
		data.islocalhost = true
	}
	var item []*connector.Connector
	var ok bool
	listener.connections.rwmux.Lock()
	if item, ok = listener.connections.connectors[ServiceName(name)]; !ok {
		item = make([]*connector.Connector, 1)
		item[0], err = connector.NewConnector(conn, data)
		if err != nil {
			l.Error("NewConnector", err)
			listener.connections.rwmux.Unlock()
			return
		}
	} else if v, err := connector.NewConnector(conn, data); err != nil {
		l.Error("NewConnector", err)
		listener.connections.rwmux.Unlock()
		return
	} else {
		item = append(item, v)
	}
	listener.connections.connectors[name] = item
	listener.connections.rwmux.Unlock()
	if isLocal {
		l.Info("Connected", name.Local())
	} else {
		l.Info("Connected", name.Remote())
	}

}

func (listener *listener) Close() error {
	return listener.listener.Close()
}
