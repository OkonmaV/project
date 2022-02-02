package main

import (
	"encoding/binary"
	"net"
	"project/test/connector"
	"project/test/epolllistener"
	"project/test/types"
	"strings"
	"time"

	"github.com/big-larry/suckutils"
)

type listener struct {
	ln *epolllistener.EpollListener
}

type listener_info struct {
	allowRemote bool

	subs     subscriptionsier
	services servicesier
	l        types.Logger
}

type listenier interface {
	close()
}

func newListener(network, address string, allowRemote bool, subs subscriptionsier, services servicesier, l types.Logger) (listenier, error) {

	lninfo := &listener_info{allowRemote: allowRemote, subs: subs, services: services, l: l}
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
	var connLocalhosted bool
	if connLocalhosted = isConnLocalhost(conn); !connLocalhosted && !lninfo.allowRemote {
		lninfo.l.Warning("HandleNewConn", suckutils.Concat("new remote conn to local-only listener from: ", conn.RemoteAddr().String(), ", conn denied"))
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

	buf = make([]byte, binary.LittleEndian.Uint32(buf))
	n, err := conn.Read(buf)
	if err != nil {
		lninfo.l.Error("HandleNewConn/Read", err)
		conn.Close()
		return
	}
	name := ServiceName(buf[:n])
	if name == ServiceName(types.ConfServiceName) {
		if connLocalhosted {
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
	if err := state.initNewConnection(conn, connLocalhosted); err != nil {
		lninfo.l.Error("HandleNewConn/initNewConnection", err)
		if _, err := conn.Write(connector.FormatBasicMessage([]byte{byte(types.OperationCodeNOTOK)})); err != nil {
			lninfo.l.Error("HandleNewConn/Send", err)
		}
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
