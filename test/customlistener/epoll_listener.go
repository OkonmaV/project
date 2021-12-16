package epolllistener

import (
	"errors"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/mailru/easygo/netpoll"
)

type EpollListener struct {
	listener        net.Listener
	desc            *netpoll.Desc
	listenerhandler ListenerHandler
	mux             sync.RWMutex
	isclosed        bool
}

var poller netpoll.Poller

type EpollErrorHandler func(error) // must start exiting the program

func SetupEpool(errhandler EpollErrorHandler) error {
	var err error
	if poller, err = netpoll.New(&netpoll.Config{OnWaitError: errhandler}); err != nil {
		return err
	}
	return nil
}

func EpollListen(network, address string, listenerhandler ListenerHandler) (*EpollListener, error) {
	if network == "unix" {
		if !strings.HasPrefix(address, "/tmp/") || !strings.HasSuffix(address, ".sock") {
			return nil, errors.New("unix address must be in form \"/tmp/[socketname].sock\"")
		}
		if err := os.RemoveAll(address); err != nil {
			return nil, err
		}
	}
	listener, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	return NewEpollListener(listener, listenerhandler)
}

func NewEpollListener(listener net.Listener, listenerhandler ListenerHandler) (*EpollListener, error) {

	if listener == nil {
		return nil, errors.New("listener is nil")
	}

	desc, err := netpoll.HandleListener(listener, netpoll.EventRead|netpoll.EventOneShot)
	if err != nil {
		return nil, err
	}

	epollln := &EpollListener{listener: listener, desc: desc, listenerhandler: listenerhandler}

	return epollln, nil
}

func (listener *EpollListener) StartServing() error {
	return poller.Start(listener.desc, listener.handle)
}

func (listener *EpollListener) handle(e netpoll.Event) {
	defer poller.Resume(listener.desc)

	listener.mux.RLock()
	defer listener.mux.RUnlock()

	conn, err := listener.listener.Accept()
	if err != nil {
		listener.listenerhandler.AcceptError(err)
		return
	}

	listener.listenerhandler.HandleNewConn(conn)
}

func (listener *EpollListener) Close() {
	listener.mux.Lock()
	defer listener.mux.Unlock()

	if listener.isclosed {
		return
	}
	listener.isclosed = true
	poller.Stop(listener.desc)
	listener.desc.Close()
	listener.listener.Close()
}

func (listener *EpollListener) IsClosed() bool {
	listener.mux.RLock()
	defer listener.mux.RUnlock()
	return listener.isclosed
}

func (listener *EpollListener) Addr() net.Addr {
	return listener.listener.Addr()
}
