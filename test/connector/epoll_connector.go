package connector

import (
	"errors"
	"fmt"
	"net"
	"project/test/gopool"
	"sync"

	"github.com/mailru/easygo/netpoll"
)

type EpollConnector struct {
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
		return errors.New("pool is already setup")
	}
	pool = gopool.NewPool(poolsize, queuesize, prespawned)
	return nil
}

func SetupEpoll(errhandler EpollErrorHandler) error {
	var err error
	if poller != nil {
		return errors.New("epoll is already setup")
	}
	if poller, err = netpoll.New(&netpoll.Config{OnWaitError: errhandler}); err != nil {
		return err
	}
	return nil
}

func NewEpollConnector(conn net.Conn, messagehandler MessageHandler) (*EpollConnector, error) {
	if conn == nil {
		return nil, ErrNilConn
	}

	desc, err := netpoll.HandleRead(conn)
	if err != nil {
		return nil, err
	}

	connector := &EpollConnector{conn: conn, desc: desc, msghandler: messagehandler}

	return connector, nil
}

func (connector *EpollConnector) StartServing() error {
	return poller.Start(connector.desc, connector.handle)
}

// MUST be called after StartServing() failure to prevent memory leak!
// (если мы  ловим ошибку в StartServing(), то мы забиваем на созданный коннектор, а его нужно закрыть, чтоб память не засирать)
func (connector *EpollConnector) ClearFromCache() {
	connector.mux.Lock()
	defer connector.mux.Unlock()

	connector.stopserving()
}

func (connector *EpollConnector) handle(e netpoll.Event) {
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

func (connector *EpollConnector) Send(message []byte) error {

	if connector.IsClosed() {
		return ErrClosedConnector
	}
	//connector.conn.SetWriteDeadline(time.Now().Add(time.Second))
	n, err := connector.conn.Write(message)
	fmt.Println("writed bytes: ", n, ", message: ", message, "|||", string(message))
	return err
}

func (connector *EpollConnector) Close(reason error) {
	connector.mux.Lock()
	defer connector.mux.Unlock()

	if connector.isclosed {
		return
	}
	connector.stopserving()
	connector.msghandler.HandleClose(reason)
}

func (connector *EpollConnector) stopserving() error {
	connector.isclosed = true
	poller.Stop(connector.desc)
	connector.desc.Close()
	return connector.conn.Close()
}

// call in HandleClose() will cause deadlock
func (connector *EpollConnector) IsClosed() bool {
	connector.mux.RLock()
	defer connector.mux.RUnlock()
	return connector.isclosed
}

func (connector *EpollConnector) RemoteAddr() net.Addr {
	return connector.conn.RemoteAddr()
}
