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
	mux          sync.RWMutex
	isclosed     bool
}

var ErrWeirdData error = errors.New("weird data")
var ErrEmptyPayload error = errors.New("empty payload")
var ErrClosedCon error = errors.New("closed connector")

type ConnectorWriter interface {
	Send([]byte) error
	ConnectorInformer
}

type ConnectorInformer interface {
	GetRemoteAddr() (string, string)
	IsClosed() bool
}

type ConnectorHandle func([]byte) error // nonnil err calls connector.Close()
type ConnectorHandleClose func(error)
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

func NewConnector(conn net.Conn, handler ConnectorHandle, closehandler ConnectorHandleClose) (*Connector, error) {

	desc, err := netpoll.HandleRead(conn)
	if err != nil {
		return nil, err
	}

	connector := &Connector{conn: conn, desc: desc, handler: handler, closehandler: closehandler}
	//poller.Start(desc, connector.handle)

	return connector, nil
}

func (connector *Connector) StartServing() error {
	return poller.Start(connector.desc, connector.handle)
}

func (connector *Connector) handle(e netpoll.Event) {
	defer poller.Resume(connector.desc)

	if e&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
		connector.Close(errors.New(e.String()))
		return
	}

	buf := make([]byte, 4)
	connector.conn.SetReadDeadline(time.Now().Add(time.Second))
	_, err := connector.conn.Read(buf)
	if err != nil {
		connector.Close(err)
		return // от паники из-за binary.BigEndian.Uint32(buf)
	}
	message_length := binary.BigEndian.Uint32(buf)

	buf = make([]byte, message_length)
	_, err = connector.conn.Read(buf)

	if pool != nil {
		pool.Schedule(func() {
			//poolwg.Add(1)
			//defer poolwg.Done()
			connector.handleinpool(buf, err)
		})
	} else {
		if err != nil {
			connector.Close(err)
			return
		}
		if err = connector.handler(buf); err != nil {
			connector.Close(err)
			return
		}
	}

}
func (connector *Connector) handleinpool(payload []byte, readerr error) {
	// connector.mux.Lock()
	// defer connector.mux.Unlock()

	connector.mux.RLock()
	if connector.IsClosed() { // повторного вызова close НЕ должно быть!
		return
	}
	connector.mux.RUnlock()

	if readerr != nil {
		connector.Close(readerr)
		return
	}

	if err := connector.handler(payload); err != nil {
		connector.Close(err)
		return
	}
}
func (connector *Connector) Send(payload []byte) error {
	// buf := make([]byte, 4)
	// binary.BigEndian.PutUint32(buf, uint32(len(message)))
	// if _, err := connector.conn.Write(buf); err != nil {
	// 	return err
	// }
	// _, err := connector.conn.Write(message)
	// if len(payload) == 0 {
	// 	return ErrEmptyPayload
	// }
	connector.mux.RLock()
	if connector.IsClosed() {
		return ErrClosedCon
	}
	connector.mux.RUnlock()

	buf := make([]byte, 4, 4+len(payload))
	binary.BigEndian.PutUint32(buf, uint32(len(payload)))
	connector.conn.SetWriteDeadline(time.Now().Add(time.Second * 2))
	_, err := connector.conn.Write(append(buf, payload...))
	return err

}

func (connector *Connector) Close(reason error) {
	connector.mux.Lock()
	defer connector.mux.Unlock()

	if connector.IsClosed() {
		return
	}
	connector.close()
	connector.closehandler(reason)
}

func (connector *Connector) close() error {
	connector.isclosed = true
	poller.Stop(connector.desc)
	connector.desc.Close()
	return connector.conn.Close()
}

func (connector *Connector) IsClosed() bool {
	return connector.isclosed
}

// network,address
func (connector *Connector) GetRemoteAddr() (string, string) {
	return connector.conn.RemoteAddr().Network(), connector.conn.RemoteAddr().String()
}

// func StopPolling() error { // CANT RUN poller.Close() from epoll.go
// 	err := poller.
// 	wgdone := make(chan struct{})
// 	go func() {
// 		poolwg.Wait()
// 		close(wgdone)
// 	}()
// 	select {
// 	case <-wgdone:
// 		return nil
// 	case <-time.After(time.Second * 5):
// 		return errors.New("")
// 	}
// }

func waiterr(err error) {
	if onwaiterr != nil {
		onwaiterr(err)
	} else {
		panic(err.Error())
	}
}
