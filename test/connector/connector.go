package connector

import (
	"encoding/binary"
	"errors"
	"log"
	"net"
	"time"

	"github.com/mailru/easygo/netpoll"
)

type Connector struct {
	conn net.Conn
	desc *netpoll.Desc
	//mux               sync.Mutex
	handler           ConnectorHandle
	disconnecthandler ConnectorHandleDisconnect
}

var ErrWeirdData error = errors.New("weird data")

type ConnectorWriter interface {
	Send([]byte) error
	ConnectorInformer
}

type ConnectorInformer interface {
	GetRemoteAddr() (string, string)
}

type ConnectorHandle func(ConnectorWriter, []byte) error
type ConnectorHandleDisconnect func(ConnectorInformer, string)

var poller netpoll.Poller

func init() {
	var err error
	if poller, err = netpoll.New(&netpoll.Config{OnWaitError: waiterr}); err != nil { // ?как onwait тут работает?
		panic("cant create poller for package \"connector\", error: " + err.Error())
	}
}

func NewConnector(conn net.Conn, handler ConnectorHandle, disconnecthandler ConnectorHandleDisconnect) (*Connector, error) {

	desc, err := netpoll.HandleRead(conn)
	if err != nil {
		return nil, err
	}
	if poller == nil {
		poller, err = netpoll.New(&netpoll.Config{OnWaitError: waiterr})
		if err != nil {
			return nil, err
		}
	}

	connector := &Connector{conn: conn, desc: desc, handler: handler, disconnecthandler: disconnecthandler}
	poller.Start(desc, connector.handle)

	return connector, nil
}

// см. тесты
// у ивентов есть свой буфер?, при дудосе новыми ивентами, после заполнения какого-то лимита новые ивенты игнорируются
func (connector *Connector) handle(e netpoll.Event) {
	defer poller.Resume(connector.desc)

	if e&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
		connector.Close()
		connector.disconnecthandler(connector, e.String())
		return
	}

	if e != netpoll.EventRead { // нужно эти ивенты логать как то
		return
	}

	connector.conn.SetReadDeadline(time.Now().Add(time.Second * 2))
	buf := make([]byte, 4)
	_, err := connector.conn.Read(buf)
	if err != nil {
		connector.Close()
		connector.disconnecthandler(connector, err.Error())
		return // от паники из-за binary.BigEndian.Uint32(buf)
	}
	message_length := binary.BigEndian.Uint32(buf)

	buf = make([]byte, message_length)
	_, err = connector.conn.Read(buf)
	if err != nil {
		connector.Close()
		connector.disconnecthandler(connector, err.Error())
		return
	}
	if err = connector.handler(connector, buf); errors.Is(err, ErrWeirdData) {
		connector.Close()
		connector.disconnecthandler(connector, err.Error())
	}
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
	poller.Stop(connector.desc)
	connector.desc.Close()
	return connector.conn.Close()

}

// network,address
func (connector *Connector) GetRemoteAddr() (string, string) {
	return connector.conn.RemoteAddr().Network(), connector.conn.RemoteAddr().String()
}

func waiterr(err error) {
	log.Panicln(err)
}
