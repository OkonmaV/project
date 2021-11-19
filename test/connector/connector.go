package connector

import (
	"encoding/binary"
	"errors"
	"net"
	"project/test/gopool"
	"sync"
	"time"

	"github.com/mailru/easygo/netpoll"
)

type Connector struct {
	conn         net.Conn
	desc         *netpoll.Desc
	handler      ConnectorHandle
	closehandler ConnectorHandleClose
	mux          sync.Mutex
	isclosed     bool
}

var ErrWeirdData error = errors.New("weird data")

type ConnectorWriter interface {
	Send([]byte) error
	ConnectorInformer
}

type ConnectorInformer interface {
	GetRemoteAddr() (string, string)
}

type ConnectorHandle func(ConnectorWriter, []byte) error // nonnil err calls connector.Close()
type ConnectorHandleClose func(ConnectorInformer, string)
type EpollErrorHandler func(error) // must start exiting the program

var poller netpoll.Poller
var pool *gopool.Pool
var onwaiterr EpollErrorHandler

func init() {
	var err error
	if poller, err = netpoll.New(&netpoll.Config{OnWaitError: waiterr}); err != nil {
		panic("cant create poller for package \"connector\", error: " + err.Error())
	}
}

// епул однопоточен, т.е. пока не обслужит первый ивент, второй ивент будет ждать
// user's handlers will be called in goroutines
func SetupGopoolHandling(poolsize, queuesize, prespawned int) error {
	if pool != nil {
		return errors.New("pool is already setupped")
	}
	pool = gopool.NewPool(poolsize, queuesize, prespawned)
	return nil
}

func SetupOnWaitErrorHandling(errhandler EpollErrorHandler) { // эта херь же будет работать?
	onwaiterr = errhandler
}

func NewConnector(conn net.Conn, handler ConnectorHandle, closehandler ConnectorHandleClose) (*Connector, error) {

	desc, err := netpoll.HandleRead(conn)
	if err != nil {
		return nil, err
	}

	connector := &Connector{conn: conn, desc: desc, handler: handler, closehandler: closehandler}
	poller.Start(desc, connector.handle)

	return connector, nil
}

func (connector *Connector) handle(e netpoll.Event) {
	defer poller.Resume(connector.desc) // ???

	if e&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
		connector.tryClose()
		connector.Close()
		connector.closehandler(connector, e.String())
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
		connector.closehandler(connector, err.Error())
		return // от паники из-за binary.BigEndian.Uint32(buf)
	}
	message_length := binary.BigEndian.Uint32(buf)

	buf = make([]byte, message_length)
	_, err = connector.conn.Read(buf)

	if pool != nil {
		pool.Schedule(func() {
			connector.handleinpool(buf, err)
		})
	} else {
		if err != nil {
			connector.Close()
			connector.closehandler(connector, err.Error())
			return
		}
		if err = connector.handler(connector, buf); err != nil {
			connector.Close()
			connector.closehandler(connector, err.Error())
		}
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

func (connector *Connector) tryClose(err error) {
	if ...
	connector.ConConnectorHandleClose
}

func (connector *Connector) Close() error {
	connector.isclosed = true
	poller.Stop(connector.desc)
	connector.desc.Close()
	return connector.conn.Close()

}

// network,address
func (connector *Connector) GetRemoteAddr() (string, string) {
	return connector.conn.RemoteAddr().Network(), connector.conn.RemoteAddr().String()
}

func (connector *Connector) handleinpool(payload []byte, readerr error) {
	connector.mux.Lock()
	defer connector.mux.Unlock()

	if connector.isclosed { // повторного вызова close НЕ должно быть!
		return
	}
	if readerr != nil {
		connector.Close()
		connector.closehandler(connector, readerr.Error())
		return
	}

	if err := connector.handler(connector, payload); err != nil {
		connector.Close()
		connector.closehandler(connector, err.Error())
	}
}

func waiterr(err error) {
	if onwaiterr != nil {
		onwaiterr(err)
	} else {
		panic(err.Error())
	}
}
