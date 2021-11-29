package main

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"project/test/connector"
	"strings"
	"sync"
	"time"

	"github.com/big-larry/suckutils"
)

type listener struct {
	listener net.Listener
	//connections *connections    а нахер он тут и ненужон, получается
}

type connections struct {
	connectors map[ServiceName][]*connector.Connector
	rwmux      sync.RWMutex
}

func (c *Configurator) NewListener(network, address string, isLocal bool) (*listener, error) {
	if network == "unix" {
		if !strings.HasPrefix(address, "/tmp/") || !strings.HasSuffix(address, ".sock") {
			return nil, errors.New("unix address must be in form \"/tmp/[socketname].sock\"")
		}
		if err := os.RemoveAll(address); err != nil {
			return nil, err
		}
	}
	ln, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	result := &listener{listener: ln /*connections: &connections{connectors: make(map[ServiceName][]*connector.Connector)}*/}
	go result.accept(c.localservices, isLocal)
	return result, nil
}

func (listener *listener) accept(localservices *servicesinfo, isLocal bool) {
	for {
		conn, err := listener.listener.Accept()
		if err != nil { // TODO: а что то кроме ошибки закрытого подключения может прилететь?
			l.Error("accept", err)
			return
		}
		conn.SetReadDeadline(time.Now().Add(time.Second * 2))
		buf := make([]byte, 4)
		_, err = conn.Read(buf)
		if err != nil {
			conn.Close()
			continue
		}

		buf = make([]byte, binary.BigEndian.Uint32(buf))
		n, err := conn.Read(buf)
		if err != nil {
			conn.Close()
			continue
		}
		name := ServiceName(buf[:n])
		if isLocal {
			if strings.Contains(conn.RemoteAddr().String(), "127.0.0.1") {
				conn.Close()
				l.Warning("locallistener", suckutils.Concat("nonlocal, introduced as \"", string(name), "\", trying to connect from ", conn.RemoteAddr().String(), ", connection refused"))
				continue
			}
			localservices.rwmux.RLock()
			if _, ok := localservices.serviceinfo[name]; !ok {
				conn.Close()
				l.Warning("locallistener", suckutils.ConcatThree("unknown trying to connect from ", conn.RemoteAddr().String(), ", connection refused"))
				localservices.rwmux.RUnlock()
				continue
			}
			localservices.rwmux.RUnlock()
		} else {
			if name != ConfServiceName {
				conn.Close()
				l.Warning("remotelistener", suckutils.ConcatThree("unknown trying to connect from ", conn.RemoteAddr().String(), ", connection refused"))
				continue
			}
		}
		coninfo := &connectorinfo{servicename: name, islocal: isLocal}
		//var con *connector.Connector
		// var item []*connector.Connector
		// var ok bool
		// listener.connections.rwmux.Lock()
		// if item, ok = listener.connections.connectors[ServiceName(name)]; !ok {
		// 	item = make([]*connector.Connector, 1)
		// 	item[0], err = connector.NewConnector(conn, coninfo.Handle, coninfo.HandleClose)
		// 	if err != nil {
		// 		listener.connections.rwmux.Unlock()
		// 		l.Error("NewConnector", err)
		// 		continue
		// 	}
		// 	coninfo.addfunctionality(item[0])
		// } else if v, err := connector.NewConnector(conn, coninfo.Handle, coninfo.HandleClose); err != nil {
		// 	listener.connections.rwmux.Unlock()
		// 	l.Error("NewConnector", err)
		// 	continue
		// } else {
		// 	coninfo.addfunctionality(v)
		// 	item = append(item, v)
		// }
		if con, err := connector.NewConnector(conn, coninfo.Handle, coninfo.HandleClose); err != nil {
			conn.Close()
			l.Error("NewConnector", err)
			continue
		} else {
			coninfo.addfunctionality(con)
			if err = con.StartServing(); err != nil { // есть риск хэндла при еще nil-овых функциях в структуре, поэтому стартуем поллинг отдельно
				l.Error("StartServing", err)
			}
			if isLocal {
				l.Info("Connected", suckutils.ConcatThree(name.Local(), " from ", conn.RemoteAddr().String()))
			} else {
				l.Info("Connected", suckutils.ConcatThree(name.Remote(), " from ", conn.RemoteAddr().String()))
			}
		}
	}
}

func (ci *connectorinfo) addfunctionality(con *connector.Connector) {
	ci.getremoteaddr = con.GetRemoteAddr
	ci.isclosedcon = con.IsClosed
	ci.send = con.Send
	ci.subscribe = func(sn []ServiceName) error {
		return ci.subscribeToServices(con, sn)
	}
}

func (listener *listener) Close() error {
	return listener.listener.Close()
}
