package httpservice

import (
	"context"
	"errors"
	"net"
	"os"
	"project/test/suspender"
	"project/test/types"
	"strings"
	"sync"
	"time"

	"github.com/big-larry/suckutils"
)

type listener struct {
	listener      net.Listener
	connsToHandle chan net.Conn

	ownStatus suspender.Suspendier
	l         types.Logger

	wg  sync.WaitGroup
	ctx context.Context

	cancelAccept bool
}

type handlefunc func(conn net.Conn) error

const handlerCallTimeout time.Duration = time.Second * 10

func newListener(ctx context.Context, l types.Logger, ownStatus suspender.Suspendier, threads int, keepAlive bool, handler handlefunc) *listener {
	ln := &listener{ctx: ctx, ownStatus: ownStatus, connsToHandle: make(chan net.Conn, 1), l: l}
	for i := 0; i < threads; i++ {
		go ln.handlingWorker(handler, keepAlive)
	}
	return ln
}

func (listener *listener) listen(network, address string) error {
	if listener != nil {
		if listener.listener.Addr().String() == address {
			return nil
		}
		listener.cancelAccept = true
		listener.close()
	}
	if network == "unix" {
		if !strings.HasPrefix(address, "/tmp/") || !strings.HasSuffix(address, ".sock") {
			return errors.New("unix address must be in form \"/tmp/[socketname].sock\"")
		}
		if err := os.RemoveAll(address); err != nil {
			return err
		}
	}
	ln, err := net.Listen(network, address)
	if err != nil {
		return err
	}

	listener.listener = ln
	listener.cancelAccept = false
	go listener.acceptWorker()

	listener.l.Info("listen", suckutils.ConcatFour("start listening at ", network, ":", address))
	return nil
}

func (listener *listener) acceptWorker() {
	for {
		conn, err := listener.listener.Accept()
		if err != nil {
			if listener.cancelAccept {
				listener.l.Debug("acceptWorker", "cancelAccept recieved, stop accept loop")
				return
			}
			listener.l.Error("acceptWorker/Accept", err)
			continue
		}
		for {
			select {
			case <-time.After(handlerCallTimeout):
				listener.l.Warning("acceptWorker/Accept", suckutils.ConcatTwo("exceeded timeout, no free handlingWorker available for ", handlerCallTimeout.String()))
			case listener.connsToHandle <- conn:
				break
			}
		}
	}
}

func (listener *listener) handlingWorker(handler handlefunc, keepAlive bool) {
	for {
		select {
		case <-listener.ctx.Done():
			return
		case conn := <-listener.connsToHandle:
			listener.wg.Add(1)
			if !listener.ownStatus.OnAir() { // TODO: куда пихнуть проверку на суспенд?
				listener.l.Warning("handlingWorker", suckutils.ConcatTwo("suspended, discard handling conn from ", conn.RemoteAddr().String()))
				conn.Close()
				continue
			}
		loop:
			for {
				select {
				case <-listener.ctx.Done():
					break loop
				default:
					if err := handler(conn); err != nil {
						listener.l.Error("handlingWorker/handle", errors.New(suckutils.ConcatThree(conn.RemoteAddr().String(), ", err: ", err.Error())))
						break loop
					}
					if !keepAlive { // TODO: кипалайв точно так должен выглядеть(спиздил у вас)? чет ебано. получается что сколько горутин, столько и подключений возможно
						break loop
					}
				}
			}
			conn.Close()
			listener.wg.Done()
		}
	}
}

func (listener *listener) close() {
	if listener.listener == nil {
		return
	}
	listener.cancelAccept = true
	if err := listener.listener.Close(); err != nil {
		listener.l.Error("listener.Close", err)
	}
	listener.listener = nil
	//listener.wg.Wait()
}
