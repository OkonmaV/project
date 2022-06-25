package appservice

import (
	"context"
	"net"
	"os"
	"project/connector"
	"project/logs/logger"
	"sync"
	"time"

	"github.com/big-larry/suckutils"
)

type listener struct {
	listener net.Listener
	appserv  *appserver

	servStatus *serviceStatus
	l          logger.Logger

	ctx context.Context

	cancelAccept     bool
	acceptWorkerDone chan struct{}
	sync.RWMutex
}

const handlerCallTimeout time.Duration = time.Second * 5
const handlerCallMaxExceededTimeouts = 3

func newListener(ctx context.Context, l logger.Logger, appserv *appserver, servStatus *serviceStatus) *listener {
	return &listener{
		ctx:          ctx,
		servStatus:   servStatus,
		appserv:      appserv,
		cancelAccept: false,
		l:            l,
	}
}

// TODO: я пока не придумал шо делать, если поднять листнер не удалось и мы ушли в суспенд (сейчас мы тупо не выйдем из суспенда)
func (listener *listener) listen(network, address string) error {
	if listener == nil {
		panic("listener.listen() called on nil listener")
	}
	listener.RLock()
	if listener.listener != nil {
		if listener.listener.Addr().String() == address {
			listener.RUnlock()
			return nil
		}
	}
	listener.RUnlock()
	listener.stop()

	listener.Lock()
	defer listener.Unlock()

	var err error
	if network == "unix" {
		if err = os.RemoveAll(address); err != nil {
			goto failure
		}
	}
	if listener.listener, err = net.Listen(network, address); err != nil {
		goto failure
	}

	listener.cancelAccept = false
	listener.acceptWorkerDone = make(chan struct{})
	go listener.acceptWorker()

	listener.servStatus.setListenerStatus(true)
	listener.l.Info("listen", suckutils.ConcatFour("start listening at ", network, ":", address))
	return nil
failure:
	listener.servStatus.setListenerStatus(false)
	return err
}

func (listener *listener) acceptWorker() {
	defer close(listener.acceptWorkerDone)
	for {
		conn, err := listener.listener.Accept()
		println("ACCEPT PASS1", err == nil) /////////////////////////////////
		if err != nil {
			if listener.cancelAccept {
				listener.l.Debug("acceptWorker", "cancelAccept recieved, stop accept loop")
				return
			}
			listener.l.Error("acceptWorker/Accept", err)
			continue
		}
		println("WAITING HERE1") /////////////////////////////////
		// listener.appserv.RLock()
		// println("ACCEPT PASS2") /////////////////////////////////
		// if listener.appserv.connAlive {
		// 	listener.appserv.RUnlock()
		// 	listener.l.Warning("acceptWorker", suckutils.ConcatTwo("conn with appserver is alive, reset accept from: ", conn.RemoteAddr().String()))
		// 	conn.Close()
		// 	continue
		// }
		// listener.appserv.RUnlock()
		println("ACCEPT PASS3") /////////////////////////////////
		if !listener.servStatus.onAir() {
			listener.l.Warning("acceptWorker", suckutils.ConcatTwo("suspended, discard handling conn from ", conn.RemoteAddr().String()))
			conn.Close()
			continue
		}
		println("ACCEPT PASS4") /////////////////////////////////
		con, err := connector.NewEpollConnector(conn, listener.appserv)
		if err != nil {
			listener.l.Error("acceptWorker/NewEpollConnector", err)
			conn.Close()
			continue
		}
		if err = con.StartServing(); err != nil {
			listener.l.Error("acceptWorker/StartServing", err)
			conn.Close()
			con.ClearFromCache()
			continue
		}
		println("ACCEPT PASS5") /////////////////////////////////
		listener.appserv.Lock()
		println("ACCEPT PASS6") /////////////////////////////////
		listener.appserv.conn = con
		listener.appserv.connAlive = true
		listener.l.Debug("acceptWorker", suckutils.ConcatTwo("connected from: ", conn.RemoteAddr().String()))
		listener.appserv.Unlock()
		println("ACCEPT PASS7") /////////////////////////////////
	}
}

// calling stop() we can call listen() again.
// и мы не ждем пока все отхэндлится
func (listener *listener) stop() {
	if listener == nil {
		panic("listener.stop() called on nil listener")
	}
	listener.Lock()
	if listener.listener == nil {
		listener.Unlock()
		return
	}

	listener.cancelAccept = true
	if err := listener.listener.Close(); err != nil {
		listener.l.Error("listener.stop()/listener.Close()", err)
	}
	<-listener.acceptWorkerDone
	listener.listener = nil

	listener.servStatus.setListenerStatus(false)
	listener.Unlock()
	listener.l.Debug("listener", "stopped")
	//listener.wg.Wait()
}

// calling close() we r closing listener forever (no further listen() calls) and waiting for all reqests to be handled
// потенциальная дыра: вызов listener.close() при keepAlive=true и НЕ завершенном контексте (см. handlingWorker())
func (listener *listener) close() {
	if listener == nil {
		panic("listener.close() called on nil listener")
	}
	listener.stop()
	println("STOPPED") ///////////////////////////////
	listener.Lock()

	listener.l.Debug("listener", "succesfully closed")
}

func (listener *listener) onAir() bool {
	listener.RLock()
	defer listener.RUnlock()
	return listener.listener != nil
}

func (listener *listener) Addr() (string, string) {
	if listener == nil {
		return "", ""
	}
	listener.RLock()
	defer listener.RUnlock()
	if listener.listener == nil {
		return "", ""
	}
	return listener.listener.Addr().Network(), listener.listener.Addr().String()
}
