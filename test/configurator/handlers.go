package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"project/test/configurator/gopool"
	"project/test/logscontainer"
	"strconv"
	"strings"
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
func (c *Configurator) handlehttp(conn net.Conn, l *logscontainer.WrappedLogsContainer, poller netpoll.Poller, pool *gopool.Pool) {

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
			l.AddTag("conn-with-serv", servicename)
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
				"x-get-addr": []string{c.services[servicename].addr},
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
		l.Error("netpoll.HandleRead", err)
		conn.Close()
		return
	}
	c.services[servicename].wsconn = conn

	if err := updateStatusToOn(c.memcConn, servicename, c.services[servicename].addr); err != nil {
		l.Error("updateStatusToOn", err)
		conn.Close()
		return
	}
	poller.Start(desc, func(ev netpoll.Event) {
		rand.Seed(time.Now().UnixNano())
		wl := l.ReWrap(map[string]string{"message-rand-id": strconv.Itoa(rand.Intn(10000))})
		if ev&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
			wl.Debug("Close wsconn", suckutils.ConcatFour("EventHup recieved from service ", servicename, " from ", conn.RemoteAddr().String()))
			poller.Stop(desc)
			conn.Close()
			if err := updateStatusToOff(c.memcConn, servicename, c.services[servicename].addr); err != nil {
				wl.Error("updateStatusToOff", err)
			}
			return
		}

		pool.Schedule(func() {
			c.services[servicename].mutex.Lock()
			c.handlews(wl, servicename, poller, desc)
			c.services[servicename].mutex.Unlock()
		})
	})

}

func (c *Configurator) handlews(l *logscontainer.WrappedLogsContainer, servicename string, poller netpoll.Poller, desc *netpoll.Desc) {
	r := wsutil.NewReader(c.services[servicename].wsconn, ws.StateServerSide)
	h, err := r.NextFrame()
	if err != nil {
		if strings.Contains(err.Error(), net.ErrClosed.Error()) { //типа костыль
			l.Warning("NextFrame", err.Error())
		} else {
			l.Error("NextFrame", err)
		}
		if err := updateStatusToOff(c.memcConn, servicename, c.services[servicename].addr); err != nil {
			l.Error("updateStatusToOff", err)
		}
		poller.Stop(desc)
		return
	}
	if h.OpCode.IsControl() {
		if err = handlecontrolframe(c.services[servicename].wsconn, h, r, ws.StateServerSide); err != nil {
			if _, ok := err.(closederr); ok {
				switch err.(closederr).code {
				case 1005:
					l.Warning("Control frame handling", "recieved close frame with no statuscode")
				case 1002:
					l.Warning("Control frame handling", "recieved close frame with protocol error (len(payload)<2)")
				}
				l.Debug("Close wsconn", suckutils.ConcatTwo("recieved ", err.Error()))
				c.services[servicename].wsconn.Close()
				poller.Stop(desc)

				if err := updateStatusToOff(c.memcConn, servicename, c.services[servicename].addr); err != nil {
					l.Error("updateStatusToOff", err)
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
			if err := updateStatusToOff(c.memcConn, servicename, c.services[servicename].addr); err != nil {
				l.Error("updateStatusToOff", err)
			}
			c.services[servicename].wsconn.Close()
			poller.Stop(desc)
			return
		}
	}

	log.Println("PAYLOAD:", string(payload)) // TODO: DELETE THIS <----------------------------------
}

func handlecontrolframe(w io.Writer, h ws.Header, r io.Reader, state ws.State) error {
	switch h.OpCode {
	case ws.OpPing:
		if h.Length == 0 {
			if h.Length == 0 {
				return ws.WriteHeader(w, ws.Header{
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

			w := wsutil.NewControlWriterBuffer(w, state, ws.OpPong, p)
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

		if _, err := io.ReadFull(r, payload); err != nil {
			return err
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
