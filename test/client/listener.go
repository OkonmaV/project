package client

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"project/test/connector"
	"strings"
	"time"

	"github.com/big-larry/suckutils"
)

type listener struct {
	listener net.Listener
	handler  ClientHandleSub
	l        logger
}

func Listen(l logger, network, address string, handler ClientHandleSub) error {
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
	listener := &listener{listener: ln, handler: handler}
	go listener.accept()
	return nil
}

func (listener *listener) accept() {
	for {
		conn, err := listener.listener.Accept()
		if err != nil {
			listener.l.Error("accept", err)
			return
		}
		conn.SetReadDeadline(time.Now().Add(time.Second * 2))
		buf := make([]byte, 4)
		_, err = conn.Read(buf)
		if err != nil {
			conn.Close()
			continue
		}

		buf = make([]byte, binary.BigEndian.Uint32(buf))
		_, err = conn.Read(buf)
		if err != nil {
			conn.Close()
			continue
		}
		name := ServiceName(buf)
		coninfo := &Connectorinfo{servicename: name}

		if con, err := connector.NewConnector(conn, coninfo.handlesub, coninfo.handlesubclose); err != nil {
			conn.Close()
			listener.l.Error("NewConnector", err)
			continue
		} else {
			coninfo.handle = listener.handler
			coninfo.getremoteaddr = con.GetRemoteAddr
			coninfo.send = con.Send
			if err = con.StartServing(); err != nil {
				listener.l.Error("StartServing", err)
			}

			listener.l.Info("Connected", suckutils.ConcatThree(string(name), " from ", conn.RemoteAddr().String()))
		}
	}
}
