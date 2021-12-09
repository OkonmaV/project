package connector_nonepoll

import (
	"errors"
	"net"
	"project/test/gopool"
	"time"
)

type Connector struct {
	conn       net.Conn
	msghandler ConnectorMessageHandler
	//mux        sync.RWMutex
	isclosed bool
}

var ErrWeirdData error = errors.New("weird data")
var ErrEmptyPayload error = errors.New("empty payload")
var ErrClosedConnector error = errors.New("closed connector")
var ErrNilConn error = errors.New("conn is nil")
var ErrNilGopool error = errors.New("gopool is nil, setup gopool first")

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

var (
	pool *gopool.Pool
	//poolwg sync.WaitGroup
)

// user's handlers will be called in goroutines
func SetupGopoolHandling(poolsize, queuesize, prespawned int) error {
	if pool != nil {
		return errors.New("pool is already setupped")
	}
	pool = gopool.NewPool(poolsize, queuesize, prespawned)
	return nil
}

func NewConnector(conn net.Conn, messagehandler ConnectorMessageHandler) (*Connector, error) {
	if conn == nil {
		return nil, ErrNilConn
	}
	if pool == nil {
		panic(ErrNilGopool)
	}

	connector := &Connector{conn: conn, msghandler: messagehandler}

	return connector, nil
}

func (connector *Connector) StartServing(sheduletimeout time.Duration, keepAlive bool) error {
	return pool.ScheduleTimeout(sheduletimeout, func() { connector.handle(keepAlive) })
}

func (connector *Connector) handle(keepAlive bool) {
	for {
		message := connector.msghandler.NewMessage()

		err := message.Read(connector.conn)
		if err != nil {
			connector.close(message, err)
			return
		}
		if err = connector.msghandler.Handle(message); err != nil {
			connector.close(message, err)
			return
		}
		if !keepAlive {
			connector.close(message, nil)
			return
		}
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
	//connector.mux.Lock()
	//defer connector.mux.Unlock()

	if connector.isclosed {
		return
	}
	connector.isclosed = true
	connector.conn.Close()
	connector.msghandler.HandleClose(message, reason)
}

// call in HandleClose() will cause deadlock
func (connector *Connector) IsClosed() bool {
	//connector.mux.RLock()
	//defer connector.mux.RUnlock()
	return connector.isclosed
}

// network,address
func (connector *Connector) GetRemoteAddr() (string, string) {
	return connector.conn.RemoteAddr().Network(), connector.conn.RemoteAddr().String()
}
