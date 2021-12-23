package main

import (
	"encoding/binary"
	"net"
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
	subs     *subscriptions
	services *services
	l        types.Logger
}

func newlistener(network, address string, subs *subscriptions, services *services, l types.Logger) (*listener, error) {

	lninfo := &listener_info{subs: subs, services: services, l: l}

	ln, err := epolllistener.EpollListen(network, address, lninfo)
	if err != nil {
		return nil, err
	}
	if err = ln.StartServing(); err != nil {
		return nil, err
	}
	lstnr := &listener{ln: ln}
	return lstnr, nil
}

func (lninfo *listener_info) HandleNewConn(conn net.Conn) {

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

}

func (lninfo *listener_info) AcceptError(err error) {
	lninfo.l.Error("Accept", err)
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
