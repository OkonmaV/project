package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"project/test/configurator/gopool"
	"project/test/logscontainer"
	"strconv"
	"time"

	"github.com/big-larry/suckutils"
	"github.com/gobwas/pool/pbytes"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/mailru/easygo/netpoll"
)

type closederr struct {
	code   uint16
	reason string
}

func (err closederr) Error() string {
	return suckutils.Concat("closeframe statuscode: ", strconv.Itoa(int(err.code)), "; reason: ", err.reason)
}

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
	if err := c.trntlConn.UpsertAsync("configurator", []interface{}{servicename, c.services[servicename].addr, true, 0}, []interface{}{
		[]interface{}{"=", "status", true},
		[]interface{}{"=", "addr", c.services[servicename].addr},
		[]interface{}{"=", "lastseen", 0},
	}).Err(); err != nil {
		l.Error("TrntlUpsert", err)
		conn.Close()
		return
	}
	//
	if err = poller.Start(desc, func(ev netpoll.Event) {
		if ev&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
			l.Debug("Close wsconn", suckutils.ConcatFour("EventHup recieved from service ", servicename, " from ", conn.RemoteAddr().String()))
			poller.Stop(desc)
			conn.Close()
			//
			if err := c.trntlConn.UpdateAsync("configurator", "primary", []interface{}{servicename}, []interface{}{
				[]interface{}{"=", "status", false},
				[]interface{}{"=", "lastseen", time.Now().Unix()},
			}).Err(); err != nil {
				l.Error("TrntlUpdate", err)
			}
			//
			return
		}
		pool.Schedule(func() {
			println(12)
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
			fmt.Println(h, "|", err)
			l.Error("NextFrame", err)
		} else {
			l.Debug("NextFrame", err.Error())
		}
		//
		if err := c.trntlConn.UpdateAsync("configurator", "primary", []interface{}{servicename}, []interface{}{
			[]interface{}{"=", "status", false},
			[]interface{}{"=", "lastseen", time.Now().Unix()},
		}).Err(); err != nil {
			l.Error("TrntlUpdate", err)
		}
		//
		poller.Stop(desc)
		return
	}
	if h.OpCode.IsControl() {
		// if err = wsutil.ControlFrameHandler(c.services[servicename].wsconn, ws.StateServerSide)(h, r); err != nil { // TODO: отказаться от wsutil
		// 	if err.(wsutil.ClosedError).Code != 1005 {
		// 		l.Error("Control frame handling", err)
		// 		l.Debug("Close wsconn", suckutils.ConcatFour("err at handling OpControl recieved from service ", servicename, " from ", c.services[servicename].wsconn.RemoteAddr().String()))
		// 	} else {
		// 		l.Debug("Close wsconn", suckutils.ConcatFour("OpClose recieved from service ", servicename, " from ", c.services[servicename].wsconn.RemoteAddr().String()))
		// 	}
		// 	//
		if err = handlecontrolframe(c.services[servicename].wsconn, h, r, ws.StateServerSide); err != nil {
			if _, ok := err.(closederr); ok {
				switch err.(closederr).code {
				case 1005:
					l.Error("Control frame handling", errors.New("recieved close frame with no statuscode"))
					l.Debug("Close wsconn", suckutils.ConcatFour("err at handling OpControl recieved from service ", servicename, " from ", c.services[servicename].wsconn.RemoteAddr().String()))
				case 1002:
					l.Error("Control frame handling", errors.New("recieved close frame with protocol error (len(payload)<2)"))
					l.Debug("Close wsconn", suckutils.ConcatFour("err at handling OpControl recieved from service ", servicename, " from ", c.services[servicename].wsconn.RemoteAddr().String()))
				default:
					l.Debug("Close wsconn", suckutils.ConcatThree(err.Error(), "; from ", c.services[servicename].wsconn.RemoteAddr().String()))
				}
				poller.Stop(desc)
				//c.services[servicename].wsconn.Close()
				if err := c.trntlConn.UpdateAsync("configurator", "primary", []interface{}{servicename}, []interface{}{
					[]interface{}{"=", "status", false},
					[]interface{}{"=", "lastseen", time.Now().Unix()},
				}).Err(); err != nil {
					l.Error("TrntlUpdate", err)
				}
			} else {
				l.Error("Control frame handling", err)
			}
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
			if err := c.trntlConn.UpdateAsync("configurator", "primary", []interface{}{servicename}, []interface{}{
				[]interface{}{"=", "status", false},
				[]interface{}{"=", "lastseen", time.Now().Unix()},
			}).Err(); err != nil {
				l.Error("TrntlUpdate", err)
			}
			//
			poller.Stop(desc)
			return
		}
	}

	log.Println("PAYLOAD:", string(payload)) // TODO: DELETE THIS <----------------------------------
}

func handlecontrolframe(c net.Conn, h ws.Header, r io.Reader, state ws.State) error {
	switch h.OpCode {
	case ws.OpPing:
		if h.Length == 0 {
			if h.Length == 0 {
				return ws.WriteHeader(c, ws.Header{
					Fin:    true,
					OpCode: ws.OpPong,
					Masked: state.ClientSide(),
				})
			}

			p := pbytes.GetLen(int(h.Length) + ws.HeaderSize(ws.Header{
				Length: h.Length,
				Masked: state.ClientSide(),
			}))
			defer pbytes.Put(p)

			w := wsutil.NewControlWriterBuffer(c, state, ws.OpPong, p)
			// if state.ServerSide() {
			// 	r = wsutil.NewCipherReader(r, h.Mask)
			// }
			_, err := io.Copy(w, r)
			if err == nil {
				err = w.Flush()
			}
			return err
		}
	case ws.OpPong:
		if h.Length == 0 {
			return nil
		}
		buf := pbytes.GetLen(int(h.Length))
		defer pbytes.Put(buf)
		_, err := io.CopyBuffer(ioutil.Discard, r, buf)
		return err
	case ws.OpClose:
		if h.Length == 0 {
			return closederr{code: 1005}
		}
		payload := make([]byte, h.Length)
		if _, err := r.Read(payload); err != nil {
			if err == io.EOF {
				err = nil
			} else {
				return err
			}
		}
		closeerr := closederr{}
		if len(payload) < 2 {
			closeerr.code = 1002
		} else {
			closeerr.code = binary.BigEndian.Uint16(payload)
			closeerr.reason = string(payload[2:])
		}
		return closeerr
	}
	return errors.New("not a control frame")
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
