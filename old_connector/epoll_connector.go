package old_connector

import (
	"errors"
	"net"
	"sync"

	"github.com/mailru/easygo/netpoll"
)

type EpollConnector[Tm any, PTm interface {
	Readable
	*Tm
}, Th MessageHandler[PTm]] struct {
	conn       net.Conn
	desc       *netpoll.Desc
	msghandler Th
	mux        sync.RWMutex
	isclosed   bool
}

func NewEpollConnector[Tmessage any,
	PTmessage interface {
		Readable
		*Tmessage
	}, Thandler MessageHandler[PTmessage]](conn net.Conn, messagehandler Thandler) (*EpollConnector[Tmessage, PTmessage, Thandler], error) {

	if conn == nil {
		return nil, ErrNilConn
	}

	desc, err := netpoll.HandleRead(conn)
	if err != nil {
		return nil, err
	}

	connector := &EpollConnector[Tmessage, PTmessage, Thandler]{conn: conn, desc: desc, msghandler: messagehandler}

	return connector, nil
}

func (connector *EpollConnector[_, _, _]) StartServing() error {
	return poller.Start(connector.desc, connector.handle)
}

// MUST be called after StartServing() failure to prevent memory leak!
// (если мы  ловим ошибку в StartServing(), то мы забиваем на созданный коннектор, а его нужно закрыть, чтоб память не засирать)
func (connector *EpollConnector[_, _, _]) ClearFromCache() {
	connector.mux.Lock()
	defer connector.mux.Unlock()

	connector.stopserving()
}

func (connector *EpollConnector[Tm, PTm, Th]) handle(e netpoll.Event) {
	defer poller.Resume(connector.desc)

	if e&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
		connector.Close(errors.New(e.String()))
		return
	}

	connector.mux.Lock() //

	if connector.isclosed {
		connector.mux.Unlock() //
		return
	}

	var err error
	message := PTm(new(Tm))
	if err = message.Read(connector.conn); err != nil {
		connector.mux.Unlock() //
		connector.Close(err)
		return
	}

	connector.mux.Unlock() //

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

func (connector *EpollConnector[_, _, _]) Send(message []byte) error {

	//connector.conn.SetWriteDeadline(time.Now().Add(time.Second))
	connector.mux.Lock()

	if connector.isclosed {
		return ErrClosedConnector
	}

	defer connector.mux.Unlock()
	_, err := connector.conn.Write(message)

	return err
}

func (connector *EpollConnector[_, _, _]) Close(reason error) {
	connector.mux.Lock()
	defer connector.mux.Unlock()

	if connector.isclosed {
		return
	}
	connector.stopserving()
	connector.msghandler.HandleClose(reason)
}

func (connector *EpollConnector[_, _, _]) stopserving() error {
	connector.isclosed = true
	poller.Stop(connector.desc)
	connector.desc.Close()
	return connector.conn.Close()
}

// call in HandleClose() will cause deadlock
func (connector *EpollConnector[_, _, _]) IsClosed() bool {
	connector.mux.RLock()
	defer connector.mux.RUnlock()
	return connector.isclosed
}

func (connector *EpollConnector[_, _, _]) RemoteAddr() net.Addr {
	return connector.conn.RemoteAddr()
}
