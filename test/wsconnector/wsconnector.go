package wsconnector

import (
	"errors"
	"net"
	"project/test/gopool"
	"sync"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/mailru/easygo/netpoll"
)

type EpollConnector struct {
	conn    net.Conn
	desc    *netpoll.Desc
	handler WsHandler
	sync.RWMutex
	isclosed bool
}

var (
	poller   netpoll.Poller
	pool     *gopool.Pool
	thisSide ws.State = ws.StateServerSide
	//otherSide ws.State = ws.StateClientSide
)

type EpollErrorHandler func(error) // must start exiting the program

var DefaultUpgrader ws.Upgrader

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
		return errors.New("epoll is already setted up")
	}
	if errhandler == nil {
		errhandler = func(e error) { panic(e) }
	}
	if poller, err = netpoll.New(&netpoll.Config{OnWaitError: errhandler}); err != nil {
		return err
	}
	return nil
}

// default: this = serverside, other = clientside
func SetupConnectionsEndpointsSides(thisEndpoint ws.State) {
	thisSide = thisEndpoint
	//otherSide = otherEndpoint
}

func CreateUpgrader(v UpgradeReqChecker) ws.Upgrader {
	return ws.Upgrader{
		OnRequest: func(uri []byte) error {
			if sc := v.CheckPath(uri); sc != 200 {
				return ws.RejectConnectionError(ws.RejectionStatus(int(sc)))
			}
			return nil
		},
		OnHost: func(host []byte) error {
			if sc := v.CheckHost(host); sc != 200 {
				return ws.RejectConnectionError(ws.RejectionStatus(int(sc)))
			}
			return nil
		},
		OnHeader: func(key, value []byte) error {
			if sc := v.CheckHeader(key, value); sc != 200 {
				return ws.RejectConnectionError(ws.RejectionStatus(int(sc)))
			}
			return nil
		},
		OnBeforeUpgrade: func() (header ws.HandshakeHeader, err error) {
			if sc := v.CheckBeforeUpgrade(); sc != 200 {
				return nil, ws.RejectConnectionError(ws.RejectionStatus(int(sc)))
			}
			return nil, nil
		},
	}
}

// upgrades connection and adds it to epoll
func NewWSConnector(upgrader ws.Upgrader, conn net.Conn, handler WsHandler) (*EpollConnector, error) {
	if conn == nil {
		return nil, ErrNilConn
	}

	if _, err := upgrader.Upgrade(conn); err != nil { // upgrade сам отправляет респонс
		return nil, err
	}

	desc, err := netpoll.HandleRead(conn)
	if err != nil {
		return nil, err
	}

	connector := &EpollConnector{conn: conn, desc: desc, handler: handler}

	return connector, nil
}

func (connector *EpollConnector) StartServing() error {
	return poller.Start(connector.desc, connector.handle)
}

// MUST be called after StartServing() failure to prevent memory leak!
func (connector *EpollConnector) ClearFromCache() {
	connector.Lock()
	defer connector.Unlock()

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

	connector.Lock()                                                //
	connector.conn.SetReadDeadline(time.Now().Add(time.Second * 5)) // TODO: for test???
	payload, _, err := wsutil.ReadData(connector.conn, thisSide)
	if err != nil {
		connector.Unlock() //
		connector.Close(err)
		return
	}
	connector.Unlock() //

	if len(payload) == 0 {
		connector.Close(ErrEmptyPayload)
		return
	}

	if pool != nil {
		pool.Schedule(func() {
			if err := connector.handler.Handle(payload); err != nil {
				connector.Close(err)
			}
		})
		return
	}
	if err = connector.handler.Handle(payload); err != nil {
		connector.Close(err)
	}
}

func (connector *EpollConnector) Send(message []byte) error {
	if connector.IsClosed() {
		return ErrClosedConnector
	}
	//connector.conn.SetWriteDeadline(time.Now().Add(time.Second))
	connector.Lock()
	defer connector.Unlock()
	return wsutil.WriteMessage(connector.conn, thisSide, ws.OpText, message)
}

func (connector *EpollConnector) Close(reason error) { // TODO: можно добавить отправку OpClose перед закрытием соединения
	connector.Lock()
	defer connector.Unlock()

	if connector.isclosed {
		return
	}
	connector.stopserving()
	connector.handler.HandleClose(reason)
}

func (connector *EpollConnector) stopserving() error {
	connector.isclosed = true
	poller.Stop(connector.desc)
	connector.desc.Close()
	return connector.conn.Close()
}

// call in HandleClose() will cause deadlock
func (connector *EpollConnector) IsClosed() bool {
	connector.RLock()
	defer connector.RUnlock()
	return connector.isclosed
}

func (connector *EpollConnector) RemoteAddr() net.Addr {
	return connector.conn.RemoteAddr()
}
