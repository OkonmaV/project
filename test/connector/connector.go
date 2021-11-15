package connector

import (
	"encoding/binary"
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/big-larry/suckutils"
	"github.com/mailru/easygo/netpoll"
)

type Connector struct {
	conn net.Conn
	desc *netpoll.Desc
	data ConnectorData
	mux  sync.Mutex
}

type ConnectorWriter interface { //TODO!
	Send([]byte) error
}

type ConnectorHandle func(ConnectorWriter, []byte) error
type ConnectorHandleDisconnect func() error

type OperationCode byte

const (
	OperationCodeSetMyStatusOff       OperationCode = 1
	OperationCodeSetMyStatusSuspended OperationCode = 2
	OperationCodeSetMyStatusOn        OperationCode = 3
	OperationCodeSubscribeToServices  OperationCode = 4
	OperationCodeSetPubAddresses      OperationCode = 5
	OperationCodeUpdatePubStatus      OperationCode = 6 // opcode + one byte for new pub's status + subscription servicename + subscription service addr
	OperationCodeError                OperationCode = 7 // must not be handled but printed at service-caller, for debugging errors in caller's code
)

var ErrNotResume error = errors.New("not resume")

var poller netpoll.Poller

func init() {
	var err error
	if poller, err = netpoll.New(&netpoll.Config{OnWaitError: waiterr}); err != nil { // ?как onwait тут работает?
		panic("cant create poller for package \"connector\", error: " + err.Error())
	}
}

func NewConnector(conn net.Conn) (*Connector, error) {

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

	connector := &Connector{conn: conn, desc: desc, data: data}
	poller.Start(desc, connector.handle)

	return connector, nil
}

func (connector *Connector) handle(e netpoll.Event) {
	//connector.mux.Lock()
	if e&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
		connector.Close()
		connector.data.HandleDisconnect(connector)
		//connector.mux.Unlock()
		return
	}

	if e != netpoll.EventRead {
		connector.data.Getlogger().Debug("Handle connector", suckutils.ConcatTwo(e.String(), " instead of EventRead"))
		poller.Resume(connector.desc)
		//connector.mux.Unlock()
		return
	}

	connector.conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	buf := make([]byte, 5)
	_, err := connector.conn.Read(buf)
	if err != nil {
		connector.data.Getlogger().Error("Read msg head", err)
		poller.Resume(connector.desc)
		//connector.mux.Unlock()
		return // от паники из-за binary.BigEndian.Uint32(buf)
	}
	message_length := binary.BigEndian.Uint32(buf)

	buf = make([]byte, message_length)
	_, err = connector.conn.Read(buf)
	//connector.mux.Unlock()
	if err != nil {
		connector.data.Getlogger().Error("Read msg", err)
	} else if err = connector.data.Handle(connector, OperationCode(buf[4]), buf); err != ErrNotResume {
		poller.Resume(connector.desc)
	}
	if err != nil {
		connector.data.Getlogger().Error("Handle", err)
	}
}

func (connector *Connector) Send(opcode OperationCode, message []byte) error {
	buf := make([]byte, 5)
	buf[4] = byte(opcode)
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
