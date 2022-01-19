package main

import (
	"encoding/binary"
	"net"
	"project/test/epolllistener"
	"project/test/suspender"
	"project/test/types"
	"strings"
	"time"

	"github.com/big-larry/suckutils"
)

type listener struct {
	ln *epolllistener.EpollListener
}

type listener_info struct {
	subs      subscriptionsier
	services  servicesier
	ownStatus suspender.Suspend_checkier
	l         types.Logger
}

type listenier interface {
	close()
}

func newListener(network, address string, subs subscriptionsier, services servicesier, l types.Logger) (listenier, error) {

	lninfo := &listener_info{subs: subs, services: services, l: l}
	ln, err := epolllistener.EpollListen(network, address, lninfo)
	if err != nil {
		return nil, err
	}
	if err = ln.StartServing(); err != nil {
		return nil, err
	}
	lninfo.l.Info("Listener", suckutils.ConcatFour("start listening at ", network, ":", address))
	lstnr := &listener{ln: ln}
	return lstnr, nil
}

// for listener's interface
func (lninfo *listener_info) HandleNewConn(conn net.Conn) {
	if !lninfo.ownStatus.OnAir() {
		lninfo.l.Debug("HandleNewConn", suckutils.ConcatTwo("suspended, discard conn from ", conn.RemoteAddr().String()))
		conn.Close()
		return
	}

	conn.SetReadDeadline(time.Now().Add(time.Second * 2))
	buf := make([]byte, 4)
	_, err := conn.Read(buf)
	if err != nil {
		lninfo.l.Error("HandleNewConn/Read", err)
		conn.Close()
		return
	}

	buf = make([]byte, binary.BigEndian.Uint32(buf))
	n, err := conn.Read(buf)
	if err != nil {
		lninfo.l.Error("HandleNewConn/Read", err)
		conn.Close()
		return
	}
	name := ServiceName(buf[:n])
	if name == ServiceName(types.ConfServiceName) {
		if isConnLocalhost(conn) {
			lninfo.l.Warning("HandleNewConn", suckutils.ConcatThree("localhosted conf trying to connect from: ", conn.RemoteAddr().String(), ", conn denied"))
			conn.Close()
			return
		}
	}

	state := lninfo.services.getServiceState(name)
	if state == nil {
		lninfo.l.Warning("HandleNewConn", suckutils.Concat("unknown service trying to connect: ", string(name)))
		conn.Close()
		return
	}
	if err := state.initNewConnection(conn); err != nil {
		lninfo.l.Error("HandleNewConn/initNewConnection", err)
		conn.Close()
		return
	}

}

// for listener's interface
func (lninfo *listener_info) AcceptError(err error) {
	lninfo.l.Error("Accept", err)
}

func (ln *listener) close() {
	ln.ln.Close() // ошибки внутри Close() не отслеживаются
}

func isConnLocalhost(conn net.Conn) bool {
	if conn.LocalAddr().Network() == "unix" {
		return true
	}
	if (conn.LocalAddr().String())[:strings.Index(conn.LocalAddr().String(), ":")] == (conn.RemoteAddr().String())[:strings.Index(conn.RemoteAddr().String(), ":")] {
		return true
	}
	return false
}
