package connector

import (
	"errors"
	"net"
	"project/test/gopool"
	"sync"
	"time"

	"github.com/mailru/easygo/netpoll"
)

type Connector struct {
	conn       net.Conn
	desc       *netpoll.Desc
	msghandler ConnectorMessageHandler
	mux        sync.RWMutex
	isclosed   bool
}

var ErrWeirdData error = errors.New("weird data")
var ErrEmptyPayload error = errors.New("empty payload")
var ErrClosedConnector error = errors.New("closed connector")
var ErrNilConn error = errors.New("conn is nil")

// for passing net.Conn
type ConnectorConnReader interface {
	Read([]byte) (int, error)
	SetReadDeadline(time.Time) error
}

// for user's implementation
type ConnectorMessageReader interface {
	Read(ConnectorConnReader) error
}

// for user's implementation
type ConnectorMessageHandler interface {
	NewMessage() ConnectorMessageReader
	Handle(ConnectorMessageReader) error
	HandleClose(ConnectorMessageReader, error)
}

// implemented by connector
type ConnectorSender interface {
	Send([]byte) error
}

// implemented by connector
type ConnectorInformer interface {
	GetRemoteAddr() (string, string)
	IsClosed() bool
}

// implemented by connector
type ConnectorCloser interface {
	Close(error)
}

//type ConnectorHandle func([]byte) error // nonnil err calls connector.Close()
//type ConnectorHandleClose func(error)
type EpollErrorHandler func(error) // must start exiting the program

var (
	poller    netpoll.Poller
	onwaiterr EpollErrorHandler
)
var (
	pool *gopool.Pool
	//poolwg sync.WaitGroup
)

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

func NewConnector(conn net.Conn, messagehandler ConnectorMessageHandler) (*Connector, error) {
	if conn == nil {
		return nil, ErrNilConn
	}

	desc, err := netpoll.HandleRead(conn)
	if err != nil {
		return nil, err
	}

	connector := &Connector{conn: conn, desc: desc, msghandler: messagehandler}

	return connector, nil
}

func (connector *Connector) StartServing() error {
	return poller.Start(connector.desc, connector.handle)
}

func (connector *Connector) handle(e netpoll.Event) {
	defer poller.Resume(connector.desc)

	message := connector.msghandler.NewMessage()

	if e&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
		connector.close(message, errors.New(e.String()))
		return
	}

	err := message.Read(connector.conn)

	if pool != nil {
		pool.Schedule(func() {
			//poolwg.Add(1)		// бессмысленно, т.к. не можем вызвать poller.Close()
			//defer poolwg.Done()
			connector.handleinpool(message, err)
		})
	} else {
		if err != nil {
			connector.close(message, err)
			return
		}
		if err = connector.msghandler.Handle(message); err != nil {
			connector.close(message, err)
			return
		}
	}
}

func (connector *Connector) handleinpool(message ConnectorMessageReader, readerr error) {

	if connector.IsClosed() {
		return
	}

	if readerr != nil {
		connector.close(message, readerr)
		return
	}

	if err := connector.msghandler.Handle(message); err != nil {
		connector.close(message, err)
		return
	}
}

func (connector *Connector) Send(message []byte) error {

	if connector.IsClosed() {
		return ErrClosedConnector
	}
	//connector.conn.SetWriteDeadline(time.Now().Add(time.Second))
	_, err := connector.conn.Write(message)
	return err
}

// will create new message - я хз как по красоте без newmessage() сделать
func (connector *Connector) Close(reason error) {
	connector.close(connector.msghandler.NewMessage(), reason)
}

func (connector *Connector) close(message ConnectorMessageReader, reason error) {
	connector.mux.Lock()
	defer connector.mux.Unlock()

	if connector.isclosed {
		return
	}
	connector.stopserving()
	connector.msghandler.HandleClose(message, reason)
}

func (connector *Connector) stopserving() error {
	connector.isclosed = true
	poller.Stop(connector.desc)
	connector.desc.Close()
	return connector.conn.Close()
}

// call in HandleClose() will cause deadlock
func (connector *Connector) IsClosed() bool {
	connector.mux.RLock()
	defer connector.mux.RUnlock()
	return connector.isclosed
}

// network,address
func (connector *Connector) GetRemoteAddr() (string, string) {
	return connector.conn.RemoteAddr().Network(), connector.conn.RemoteAddr().String()
}

func waiterr(err error) {
	if onwaiterr != nil {
		onwaiterr(err)
	} else {
		panic(err.Error())
	}
}
