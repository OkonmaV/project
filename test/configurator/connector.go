package main

import (
	"encoding/binary"
	"log"
	"net"
	"sync"

	"github.com/big-larry/suckutils"
	"github.com/mailru/easygo/netpoll"
)

type Connector struct {
	name    ServiceName
	conn    net.Conn
	desc    *netpoll.Desc
	poller  netpoll.Poller
	handler func(*Connector, []byte)
	mux     sync.Mutex
}

type Listener struct {
	listener    net.Listener
	connections *connections
}

type connections struct {
	connectors map[ServiceName][]*Connector
	rwmux      sync.RWMutex
}

func (c *connections) Remove(servicename ServiceName, connector *Connector) { // TODO: прикрутить нужно куда то
	c.rwmux.Lock()
	for i, cnn := range c.connectors[servicename] {
		if connector == cnn {
			copy(c.connectors[servicename][i:], c.connectors[servicename][i+1:])
			c.connectors[servicename] = c.connectors[servicename][:len(c.connectors[servicename])-1]
		}
	}
}

func NewConnector(name ServiceName, conn net.Conn, handler func(*Connector, []byte)) (*Connector, error) {
	desc, err := netpoll.HandleRead(conn)
	if err != nil {
		return nil, err
	}

	poller, err := netpoll.New(&netpoll.Config{OnWaitError: waiterr}) // ПОЧЕМУ НОВЫЙ ПОЛЛЕР А НЕ ПРОКИДЫВАТЬ ОДИН И ТОТ ЖЕ??
	if err != nil {
		return nil, err
	}

	connector := &Connector{name: name, conn: conn, poller: poller, desc: desc, handler: handler}
	poller.Start(desc, connector.handle)

	return connector, nil
}

func (connector *Connector) handle(e netpoll.Event) {
	connector.mux.Lock()
	defer connector.mux.Unlock()
	l.Debug("handle", suckutils.ConcatThree(string(connector.name), " event: ", e.String()))
	if e&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
		l.Debug("Disconnected", suckutils.Concat(string(connector.name), " reason:", e.String()))
		if err := connector.Close(); err != nil {
			l.Error("Close connector", err)
		}
		return
	}
	if e != netpoll.EventRead {
		return
	}
	buf := make([]byte, 4)
	n, err := connector.conn.Read(buf)
	if err != nil {
		l.Error("Read", err)
	}
	message_length := binary.BigEndian.Uint32(buf)
	buf = make([]byte, message_length)
	n, err = connector.conn.Read(buf)
	if err != nil {
		l.Error(string(connector.name), err)
	} else {
		log.Println(connector.name, string(buf[:n]))
	}
	// if data, err := ioutil.ReadAll(connector.conn); err != nil {
	// 	log.Println(err)
	// } else {
	// 	log.Println(string(data))
	// }
	connector.poller.Resume(connector.desc)
}

func waiterr(err error) {
	log.Panicln(err)
}

func (connector *Connector) Send(message []byte) error {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(len(message)))
	if _, err := connector.conn.Write(buf); err != nil {
		return err
	}
	_, err := connector.conn.Write(message)
	return err
}

func (connector *Connector) Close() error {
	connector.poller.Stop(connector.desc)
	connector.desc.Close()
	return connector.conn.Close()

}

func NewListener(network, address string, handler func(*Connector, []byte)) (*Listener, error) {
	// if network == "unix" {
	// 	if err := os.RemoveAll(address); err != nil {
	// 		return nil, err
	// 	}
	// }
	listener, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	result := &Listener{listener: listener, connections: &connections{connectors: make(map[ServiceName][]*Connector)}}
	go result.accept(handler)
	return result, nil
}

func (listener *Listener) accept(handler func(*Connector, []byte)) {
	conn, err := listener.listener.Accept()
	if err != nil {
		log.Println(err)
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
	// TODO: HERE CHECK SERVICENAME IN MEMC AND UPDATE ITS STATUS

	var item []*Connector
	var ok bool
	listener.connections.rwmux.Lock()
	if item, ok = listener.connections.connectors[ServiceName(name)]; !ok {
		item = make([]*Connector, 1)
		item[0], err = NewConnector(name, conn, handler)
		if err != nil {
			log.Println(err)
		}
	} else if v, err := NewConnector(name, conn, handler); err != nil {
		log.Println(err)
	} else {
		item = append(item, v)
	}
	listener.connections.connectors[name] = item
	listener.connections.rwmux.Unlock()

	if listener.listener.Addr().Network() == "unix" { // TODO: да, я знаю что херово выглядит
		l.Info("Connected", name.Local())
	} else {
		l.Info("Connected", name.Remote())
	}

}

func (listener *Listener) Close() error {
	return listener.listener.Close()
}
