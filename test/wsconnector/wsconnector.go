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

type EpollWSConnector struct {
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

// default: thisEndpoint = serverside
func SetupConnectionsEndpointSide(thisEndpoint ws.State) {
	thisSide = thisEndpoint
	//otherSide = otherEndpoint
}

func createUpgrader(v UpgradeReqChecker) ws.Upgrader {
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
func NewWSConnector(conn net.Conn, handler WsHandler) (*EpollWSConnector, error) {
	if conn == nil {
		return nil, ErrNilConn
	}

	if _, err := createUpgrader(handler).Upgrade(conn); err != nil { // upgrade сам отправляет респонс
		return nil, err
	}

	desc, err := netpoll.HandleRead(conn)
	if err != nil {
		return nil, err
	}

	connector := &EpollWSConnector{conn: conn, desc: desc, handler: handler}

	return connector, nil
}

func (connector *EpollWSConnector) StartServing() error {
	return poller.Start(connector.desc, connector.handle)
}

// MUST be called after StartServing() failure to prevent memory leak!
func (connector *EpollWSConnector) ClearFromCache() {
	connector.Lock()
	defer connector.Unlock()

	connector.stopserving()
}

func (connector *EpollWSConnector) handle(e netpoll.Event) {
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
	h, r, err := wsutil.NextReader(connector.conn, thisSide)
	if err != nil {
		connector.Unlock() //
		connector.Close(err)
		return
	}
	if h.OpCode.IsControl() {
		if err := wsutil.ControlFrameHandler(connector.conn, thisSide)(h, r); err != nil {
			connector.Unlock() //
			connector.Close(err)
		}
		return
	}
	println("AAA")
	time.Sleep(time.Second)
	message := connector.handler.NewMessage()
	if err := message.Read(r, h); err != nil {
		connector.Unlock() //
		connector.Close(err)
	}
	// payload, _, err := wsutil.ReadData(connector.conn, thisSide)
	// if err != nil {
	// 	connector.Unlock() //
	// 	connector.Close(err)
	// 	return
	// }
	connector.Unlock() //

	// if len(payload) == 0 {
	// 	connector.Close(ErrEmptyPayload)
	// 	return
	// }

	if pool != nil {
		pool.Schedule(func() {
			if err := connector.handler.Handle(message); err != nil {
				connector.Close(err)
			}
		})
		return
	}
	if err = connector.handler.Handle(message); err != nil {
		connector.Close(err)
	}
}

func (connector *EpollWSConnector) Send(message []byte) error {
	if connector.IsClosed() {
		return ErrClosedConnector
	}
	//connector.conn.SetWriteDeadline(time.Now().Add(time.Second))
	connector.Lock()
	defer connector.Unlock()
	return wsutil.WriteMessage(connector.conn, thisSide, ws.OpText, message)
}

func (connector *EpollWSConnector) Close(reason error) { // TODO: можно добавить отправку OpClose перед закрытием соединения
	connector.Lock()
	defer connector.Unlock()

	if connector.isclosed {
		return
	}
	connector.stopserving()
	connector.handler.HandleClose(reason)
}

func (connector *EpollWSConnector) stopserving() error {
	connector.isclosed = true
	poller.Stop(connector.desc)
	connector.desc.Close()
	return connector.conn.Close()
}

// call in HandleClose() will cause deadlock
func (connector *EpollWSConnector) IsClosed() bool {
	connector.RLock()
	defer connector.RUnlock()
	return connector.isclosed
}

func (connector *EpollWSConnector) RemoteAddr() net.Addr {
	return connector.conn.RemoteAddr()
}
