package connector

import (
	"errors"
	"net"
	"project/test/gopool"
	"sync"

	"github.com/mailru/easygo/netpoll"
)

type EpoolConnector struct {
	conn       net.Conn
	desc       *netpoll.Desc
	msghandler MessageHandler
	mux        sync.RWMutex
	isclosed   bool
}

var (
	poller netpoll.Poller
	pool   *gopool.Pool
)

type EpollErrorHandler func(error) // must start exiting the program

// епул однопоточен, т.е. пока не обслужит первый ивент, второй ивент будет ждать
// user's handlers will be called in goroutines
func SetupGopoolHandling(poolsize, queuesize, prespawned int) error {
	if pool != nil {
		return errors.New("pool is already setupped")
	}
	pool = gopool.NewPool(poolsize, queuesize, prespawned)
	return nil
}

func SetupEpool(errhandler EpollErrorHandler) error {
	var err error
	if poller, err = netpoll.New(&netpoll.Config{OnWaitError: errhandler}); err != nil {
		return err
	}
	return nil
}

func NewEpoolConnector(conn net.Conn, messagehandler MessageHandler) (*EpoolConnector, error) {
	if conn == nil {
		return nil, ErrNilConn
	}

	desc, err := netpoll.HandleRead(conn)
	if err != nil {
		return nil, err
	}

	connector := &EpoolConnector{conn: conn, desc: desc, msghandler: messagehandler}

	return connector, nil
}

func (connector *EpoolConnector) StartServing() error {
	return poller.Start(connector.desc, connector.handle)
}

func (connector *EpoolConnector) handle(e netpoll.Event) {
	defer poller.Resume(connector.desc)

	if e&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
		connector.Close(errors.New(e.String()))
		return
	}

	if connector.IsClosed() {
		return
	}
	var err error
	message := connector.msghandler.NewMessage()
	if err = message.Read(connector.conn); err != nil {
		connector.Close(err)
		return
	}
	if pool != nil {
		pool.Schedule(func() {
			if err := connector.msghandler.Handle(message); err != nil {
				connector.Close(err)
			}
		})
		return
	}
	if err = connector.msghandler.Handle(message); err != nil {
		connector.Close(err)
	}
}

func (connector *EpoolConnector) Send(message []byte) error {

	if connector.IsClosed() {
		return ErrClosedConnector
	}
	//connector.conn.SetWriteDeadline(time.Now().Add(time.Second))
	_, err := connector.conn.Write(message)
	return err
}

// will create new message - я хз как по красоте без newmessage() сделать
func (connector *EpoolConnector) Close(reason error) {
	connector.mux.Lock()
	defer connector.mux.Unlock()

	if connector.isclosed {
		return
	}
	connector.stopserving()
	connector.msghandler.HandleClose(reason)
}

func (connector *EpoolConnector) stopserving() error {
	connector.isclosed = true
	poller.Stop(connector.desc)
	connector.desc.Close()
	return connector.conn.Close()
}

// call in HandleClose() will cause deadlock
func (connector *EpoolConnector) IsClosed() bool {
	connector.mux.RLock()
	defer connector.mux.RUnlock()
	return connector.isclosed
}

// network,address
func (connector *EpoolConnector) GetRemoteAddr() (string, string) {
	return connector.conn.RemoteAddr().Network(), connector.conn.RemoteAddr().String()
}
