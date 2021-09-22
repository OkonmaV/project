package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"project/test/auth/logscontainer"
	"project/test/configurator/gopool"

	"github.com/big-larry/suckutils"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/mailru/easygo/netpoll"
)

func (c *Configurator) handlehttp(conn net.Conn, l *logscontainer.LogsContainer, poller netpoll.Poller, pool *gopool.Pool) {

	var servicename string

	u := ws.Upgrader{
		OnRequest: func(uri []byte) error {
			servicename = string(uri[1:])
			if _, ok := c.services[servicename]; !ok {
				l.Debug("Upgrade request", suckutils.ConcatThree("Unknown servicename \"", servicename, "\""))
				return ws.RejectConnectionError(
					ws.RejectionStatus(403),
				)
			}
			return nil
		},
		OnHost: func(host []byte) error {
			ind := bytes.Index(host, []byte{58}) // for cutting port, 58=":"
			if _, ok := c.hosts[string(host[:ind])]; !ok {
				l.Warning("Upgrade request", suckutils.ConcatThree("Unknown host \"", string(host), "\""))
				return ws.RejectConnectionError(
					ws.RejectionStatus(403),
				)
			}
			return nil
		},
		OnBeforeUpgrade: func() (header ws.HandshakeHeader, err error) {
			if c.services[servicename].addr == "*" {
				if c.services[servicename].addr, err = getfreeaddr(); err != nil {
					l.Error("GetFreePort", err)
					ws.RejectConnectionError(
						ws.RejectionStatus(500),
					)
				}

			}
			return ws.HandshakeHeaderHTTP(http.Header{ // TODO: delete http
				"X-listen-here-u-little-shit": []string{c.services[servicename].addr},
			}), nil
		},
	}

	_, err := u.Upgrade(conn)
	if err != nil {
		l.Error("Upgrade", errors.New(suckutils.ConcatFour("Upgrading connection from ", conn.RemoteAddr().String(), " error: ", err.Error())))
		conn.Close()
		return
	}

	l.Debug("Established new wsconn", suckutils.ConcatFour("Service ", servicename, " from ", conn.RemoteAddr().String()))

	desc, err := netpoll.HandleRead(conn)
	if err != nil {
		l.Error("Creating wsconn descriptor ", err)
		conn.Close()
		return
	}
	//
	c.services[servicename].wsconn = conn
	c.servicesStatus[servicename] = 1
	//
	if err = poller.Start(desc, func(ev netpoll.Event) {
		if ev&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
			l.Debug("Close wsconn", suckutils.ConcatFour("EventHup recieved from service ", servicename, " from ", conn.RemoteAddr().String()))
			poller.Stop(desc)
			conn.Close()
			//
			c.servicesStatus[servicename] = 0
			//
			return
		}

		pool.Schedule(func() {
			c.services[servicename].mutex.Lock()
			c.handlews(l, servicename, poller, desc)
			c.services[servicename].mutex.Unlock()
		})
	}); err != nil {
		fmt.Println("Starting poller err:", err)
		conn.Close()
	}
}

func (c *Configurator) handlews(l *logscontainer.LogsContainer, servicename string, poller netpoll.Poller, desc *netpoll.Desc) {
	r := wsutil.NewReader(c.services[servicename].wsconn, ws.StateServerSide)
	h, err := r.NextFrame()
	if err != nil {
		if err != net.ErrClosed {
			l.Error("NextFrame", err)
		} else {
			l.Debug("NextFrame", err.Error())
		}
		//
		//c.services[servicename].wsconn.Close()
		c.servicesStatus[servicename] = 0
		//
		poller.Stop(desc)
		return
	}
	if h.OpCode.IsControl() {
		if err = wsutil.ControlFrameHandler(c.services[servicename].wsconn, ws.StateServerSide)(h, r); err != nil { // TODO: отказаться от wsutil
			if err.(wsutil.ClosedError).Code != 1005 {
				l.Error("Control frame handling", err)
				l.Debug("Close wsconn", suckutils.ConcatFour("err at handling OpControl recieved from service ", servicename, " from ", c.services[servicename].wsconn.RemoteAddr().String()))
			} else {
				l.Debug("Close wsconn", suckutils.ConcatFour("OpClose recieved from service ", servicename, " from ", c.services[servicename].wsconn.RemoteAddr().String()))
			}
			//
			//c.services[servicename].wsconn.Close()
			c.servicesStatus[servicename] = 0
			//
			poller.Stop(desc)
			return
		}
		return
	}
	payload := make([]byte, h.Length)
	if _, err = r.Read(payload); err != nil {
		if err == io.EOF {
			err = nil
		} else {
			l.Error("Reading payload", err)
			//
			//c.services[servicename].wsconn.Close()
			c.servicesStatus[servicename] = 0
			//
			poller.Stop(desc)
			return
		}
	}

	log.Println("PAYLOAD:", string(payload)) // TODO: DELETE THIS <----------------------------------
}

func getfreeaddr() (string, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return "", err
	}
	defer l.Close()
	return l.Addr().String(), nil
}
