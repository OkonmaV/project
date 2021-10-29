package connector

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"

	"github.com/mailru/easygo/netpoll"
)

type Connector struct {
	name    string
	conn    net.Conn
	desc    *netpoll.Desc
	poller  netpoll.Poller
	handler func([]byte)
}

func NewConnector(name string, conn net.Conn, handler func([]byte)) (*Connector, error) {
	desc, err := netpoll.HandleRead(conn)
	if err != nil {
		return nil, err
	}

	poller, err := netpoll.New(&netpoll.Config{OnWaitError: waiterr})
	if err != nil {
		return nil, err
	}

	connector := &Connector{name: name, conn: conn, poller: poller, desc: desc, handler: handler}
	poller.Start(desc, connector.handle)

	return connector, nil
}

func (connector *Connector) handle(e netpoll.Event) {
	log.Println("handle", connector.name, e)
	if e != netpoll.EventRead {
		return
	}
	buf := make([]byte, 4)
	n, err := connector.conn.Read(buf)
	if err != nil {
		log.Println(connector.name, string(buf[:n]))
	}
	message_length := binary.BigEndian.Uint32(buf)
	buf = make([]byte, message_length)
	n, err = connector.conn.Read(buf)
	if err != nil {
		log.Println(connector.name, err)
	} else {
		log.Println(connector.name, string(buf[:n]))
		connector.handler(buf)
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

type Listener struct {
	listener    net.Listener
	Connections map[string][]*Connector
}

func NewListener(network, address string) (*Listener, error) {
	listener, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	result := &Listener{listener: listener, Connections: make(map[string][]*Connector)}
	go result.accept()
	return result, nil
}

func (listener *Listener) accept() {
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
	name := string(buf[:n])

	var item []*Connector
	var ok bool
	if item, ok = listener.Connections[name]; !ok {
		item = make([]*Connector, 1)
		item[0], err = NewConnector("server for "+name, conn, func(message []byte) {
			fmt.Println(string(message))
		})
		if err != nil {
			log.Println(err)
		}
	} else if v, err := NewConnector("server for "+name, conn, func(message []byte) {
		fmt.Println(string(message))
	}); err != nil {
		log.Println(err)
	} else {
		item = append(item, v)
	}
	listener.Connections[name] = item
	log.Println("Connected", name)
}

func (listener *Listener) Close() error {
	return listener.listener.Close()
}
