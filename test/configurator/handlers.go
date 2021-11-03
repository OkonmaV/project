package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"project/test/configurator/gopool"
	"project/test/logscontainer"
	"strconv"
	"strings"
	"time"

	"github.com/big-larry/suckutils"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/gobwas/pool/pbytes"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/mailru/easygo/netpoll"
)

func (c *Configurator) handleHTTP(conn net.Conn, l *logscontainer.WrappedLogsContainer, poller netpoll.Poller, pool *gopool.Pool) {

	servinfo := &serviceinfo{}
	// TODO: ПОДУМАТЬ НА ТЕМУ - ЧТО БУДЕТ ЕСЛИ ПОДРУБИТСЯ СЕРВИС С ТАКИМ ЖЕ ИМЕНЕМ
	u := ws.Upgrader{
		OnRequest: func(uri []byte) error {
			servinfo.name = ServiceName(uri[1:]) // skip "/"
			if _, ok := c.localservices[servinfo.name]; !ok {
				if servinfo.name != ConfServiceName {
					l.Debug("Upgrade request", suckutils.ConcatThree("Unknown servicename \"", string(servinfo.name), "\", connection rejected"))
					return ws.RejectConnectionError(
						ws.RejectionStatus(403),
					)
				}
			}
			return nil
		},
		OnHost: func(host []byte) error {
			ind := bytes.Index(host, []byte{58}) // for cutting port, 58=":"
			if string(host[:ind]) != "127.0.0.1" {
				servinfo.isRemote = true
			}
			return nil
		},
		OnBeforeUpgrade: func() (header ws.HandshakeHeader, err error) { // header or err non-nil, never both
			if servinfo.name != ConfServiceName {
				if servinfo.isRemote { // REJECT NONCONFIG'to'CONFIG REMOTE CONNECTION
					l.Warning("Upgrade request", suckutils.ConcatThree("remote service ", servinfo.nameWithLocationType(), " trying to connect, connection rejected"))
					err = ws.RejectConnectionError(
						ws.RejectionStatus(403),
					)
					return
				}
				if c.localservices[servinfo.name].addr == nil {
					if c.localservices[servinfo.name].addr, err = getfreeaddr(); err != nil {
						l.Error("GetFreePort", err)
						err = ws.RejectConnectionError(
							ws.RejectionStatus(500),
						)
						return
					}
				}
			} else if !servinfo.isRemote {
				l.Warning("Upgrade request", suckutils.ConcatThree("localhosted ", string(ConfServiceName), " want to connect to localhosted brother, ws connection rejected"))
				err = ws.RejectConnectionError(
					ws.RejectionStatus(403),
				)
				return
			}

			header = ws.HandshakeHeaderHTTP(http.Header{ // TODO: test conf'to'conf connection
				"x-get-addr": []string{c.localservices[servinfo.name].addr.String()}},
			)
			return
		},
	}

	if _, err := u.Upgrade(conn); err != nil {
		l.Error("Upgrade", errors.New(suckutils.ConcatFour("Upgrading connection from ", conn.RemoteAddr().String(), " error: ", err.Error())))
		conn.Close()
		return
	}

	desc, err := netpoll.HandleRead(conn)
	if err != nil {
		l.Error("netpoll.HandleRead", err)
		conn.Close()
		return
	}

	l.Debug("Established new wsconn", suckutils.ConcatFour("Service ", servinfo.nameWithLocationType(), " from ", conn.RemoteAddr().String()))
	l.SetTag(logscontainer.TagNameOfConnectedService, servinfo.nameWithLocationType())

	if servinfo.name == ConfServiceName {
		confinstance := &serviceinstance{wsconn: conn} // TODO: КАК ОТРУБАТЬ ХОСТ ПРИ ОТВАЛЕ КОНФИГУРАТОРА????
		poller.Start(desc, func(ev netpoll.Event) {
			rand.Seed(time.Now().UnixNano())
			wl := l.ReWrap(map[logscontainer.Tag]string{logscontainer.TagRandEventId: strconv.Itoa(rand.Intn(10000))})
			if ev&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
				wl.Debug("Close wsconn", suckutils.ConcatFour("EventHup recieved from service ", servinfo.nameWithLocationType(), " from ", conn.RemoteAddr().String()))
				poller.Stop(desc)
				conn.Close()
				// отвал
				return
			}
			pool.Schedule(func() {
				confinstance.mutex.Lock()
				c.handleConfiguratoreWS(wl, servinfo.name, confinstance, poller, desc)
				confinstance.mutex.Unlock()
			})
		})

	}
	if err := c.updateServiceStatus(l, servinfo.name, StatusSuspended); err != nil {
		l.Error("updateStatus", err)
		conn.Close()
		return
	}
	// TODO: test when configurator closes connection
	poller.Start(desc, func(ev netpoll.Event) {
		rand.Seed(time.Now().UnixNano())
		wl := l.ReWrap(map[logscontainer.Tag]string{logscontainer.TagRandEventId: strconv.Itoa(rand.Intn(10000))})
		if ev&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
			wl.Debug("Close wsconn", suckutils.ConcatFour("EventHup recieved from service ", servinfo.nameWithLocationType(), " from ", conn.RemoteAddr().String()))
			poller.Stop(desc)
			conn.Close()
			if err := c.updateServiceStatus(l, servinfo.name, StatusOff); err != nil {
				wl.Error("updateStatus", err)
			}
			return
		}

		pool.Schedule(func() {
			c.localservices[servinfo.name].mutex.Lock()
			c.handleServiceWS(wl, servinfo.name, c.localservices[servinfo.name], poller, desc)
			c.localservices[servinfo.name].mutex.Unlock()
		})
	})

}

func (c *Configurator) handlews(connector *Connector, payload []byte) {
	r := wsutil.NewReader(servinstance.wsconn, ws.StateServerSide)
	h, err := r.NextFrame()
	if err != nil {
		if strings.Contains(err.Error(), net.ErrClosed.Error()) { //типа костыль
			l.Debug("NextFrame", err.Error())
		} else {
			l.Error("NextFrame", err)
		}
		if err := c.updateServiceStatus(l, servicename, StatusOff); err != nil {
			l.Error("updateStatus", err)
		}
		servinstance.wsconn.Close()
		poller.Stop(desc)
		return
	}
	if h.OpCode.IsControl() {
		if err = handlecontrolframe(servinstance.wsconn, h, r, ws.StateServerSide); err != nil {
			if _, ok := err.(closederr); ok {
				switch err.(closederr).code {
				case 1005:
					l.Warning("Control frame handling", "recieved close frame with no statuscode")
				case 1002:
					l.Warning("Control frame handling", "recieved close frame with protocol error (len(payload)<2)")
				}
				servinstance.wsconn.Close()
				poller.Stop(desc)
				l.Debug("Close wsconn", suckutils.ConcatTwo("recieved ", err.Error()))

				if err := c.updateServiceStatus(l, servicename, StatusOff); err != nil {
					l.Error("updateStatus", err)
				}
			} else {
				servinstance.wsconn.Close()
				poller.Stop(desc)
				l.Error("Control frame handling", err)
			}
			return
		}
		return
	}
	if h.Length == 0 {
		l.Warning("Frame", "payload is zero length")
		return
	}
	payload := make([]byte, h.Length)
	if _, err = r.Read(payload); err != nil {
		if err == io.EOF {
			err = nil
		} else {
			l.Error("Reading payload", err)
			if err := c.updateServiceStatus(l, servicename, StatusOff); err != nil {
				l.Error("updateStatus", err)
			}
			servinstance.wsconn.Close()
			poller.Stop(desc)
			return
		}
	}
	switch OperationCode(payload[0]) {
	case OperationCodeSetMyStatusSuspended:
		l.Debug("Frame", "setting status to suspended")
		if err := c.updateServiceStatus(l, servicename, StatusSuspended); err != nil {
			l.Error("updateStatus", err)
		}
		return
	case OperationCodeSetMyStatusOn:
		l.Debug("Frame", "setting status to on")
		if err := c.updateServiceStatus(l, servicename, StatusOn); err != nil {
			l.Error("updateStatus", err)
			poller.Stop(desc)
			servinstance.wsconn.Close()
		}
		return
	case OperationCodeSubscribeToServices:
		servicenames := strings.Split(string(payload[1:]), "/")
		for i, servicename := range servicenames {
			if servicename == "" {
				if err := SendMessage(servinstance.wsconn, ws.StateServerSide, OperationCodeError, []byte("opcode protocol error")); err != nil {
					l.Error("SendMessage", err)
					poller.Stop(desc)
					servinstance.wsconn.Close()
				}
				return
			}
			servicenames[i] = ServiceName(servicename).LocalSub()
		}
		if err = subscribeToServices(l, c.memcConn, string(servicename), servicenames); err != nil {
			l.Error("subscribeToServices", err)
			poller.Stop(desc)
			servinstance.wsconn.Close()
		}
		l.Debug("Frame", "subscribed to some services")
		addresses, err := getAllServiceAddresses(c.memcConn, ServiceName(servicename))
		if err != nil {
			l.Error("getAllServiceAddresses", err)
			poller.Stop(desc)
			servinstance.wsconn.Close()
		}
		if err = SendMessage(servinstance.wsconn, ws.StateServerSide, OperationCodeSetPubAddresses, addresses); err != nil {
			poller.Stop(desc)
			servinstance.wsconn.Close()

		}
		l.Debug("Frame", "sended first OperationCodeUpdateSubs")
	}

	l.Info("PAYLOAD", string(payload)) // TODO: DELETE THIS <----------------------------------
}

// TODO: че с ремоутами?
// Subscribe "subservicename" to services from "pubservicenames".
// subservicename must be raw,
// pubservicenames must be in form "local/remote.sub.servicename"
func subscribeToServices(l *logscontainer.WrappedLogsContainer, memcConn *memcache.Client, subservicename string, pubservicenames []string) error {
	if subservicename == "" || len(pubservicenames) == 0 {
		return errors.New("servicenames params must not be nil")
	}
	items, err := memcConn.GetMulti(pubservicenames)
	if err != nil {
		return err
	}

	subserv := []byte(subservicename)
	for _, pubservname := range pubservicenames {
		if item, ok := items[pubservname]; ok { // 1 если запись с подписками есть
			if len(item.Value) != 0 { // 2 если уже есть подписчики
				subs := bytes.Split(item.Value, []byte{47}) // 47="/"
				var exists bool
				for _, sub := range subs {
					if bytes.Equal(sub, subserv) {
						exists = true
						break
					}
				}
				if !exists { // 3 если еще не подписан
					newValue := make([]byte, 0, len(item.Value)+len(subserv)+1)
					item.Value = append(append(append(newValue, subserv...), 47), item.Value...)
					if err = memcConn.Set(item); err != nil {
						return err
					}
				} else { // 3 если уже подписан
					l.Warning("subsctibeToServices", suckutils.ConcatThree("service repeatedly trying to subscribe to service \"", pubservname, "\""))
				}
			} else { // 2 если запись есть, но подписчиков нет
				item.Value = subserv
				if err = memcConn.Set(item); err != nil {
					return err
				}
			}
		} else { // 1 если записи с подписками нет
			if err = memcConn.Set(&memcache.Item{Key: pubservname, Value: subserv}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Configurator) sendToMany(l *logscontainer.WrappedLogsContainer, payload []byte, recievers []*serviceinstance) {
	for _, reciever := range recievers {
		if reciever.wsconn != nil {
			reciever.mutex.Lock()
			if err := ws.WriteFrame(reciever.wsconn, ws.NewBinaryFrame(payload)); err != nil {
				l.Warning("sendToMany", suckutils.ConcatFour("SendMessage to ", reciever.addr.String(), "error: ", err.Error()))
				l.Debug(reciever.addr.String(), "disconnected because of SendMessage error")
				reciever.wsconn.Close() // обсуждаемо, но утвержаю
			}
			reciever.mutex.Unlock()
		} else {
			l.Warning("sendToMany", suckutils.Concat("wsconn to ", reciever.addr.String(), " is nil, skipped"))
		}
	}
}

func (c *Configurator) updateServiceStatus(l *logscontainer.WrappedLogsContainer, servicename ServiceName, newstatus ServiceStatus) error {
	if err := c.updateMemcServiceStatus(l, servicename, newstatus); err != nil {
		return errors.New(suckutils.ConcatTwo("updateMemcServiceStatus err: ", err.Error()))
	}
	subs, err := c.getAllSubs(l, servicename)
	if err != nil {
		return errors.New(suckutils.ConcatTwo("getAllSubs err: ", err.Error()))
	}
	payload := append(make([]byte, 0, len(servicename)+7), byte(StatusOn), byte(OperationCodeUpdatePubStatus))
	payload = append(payload, []byte(servicename)...)
	payload = append(payload, c.localservices[servicename].addr...)
	c.sendToMany(l, payload, subs)
	return nil
}

// TODO: ADD REMOTE
func (c *Configurator) getAllSubs(l *logscontainer.WrappedLogsContainer, servicename ServiceName) ([]*serviceinstance, error) {
	if servicename == "" {
		return nil, errors.New("servicename must not be empty")
	}
	localkey := servicename.LocalSub()
	remotekey := servicename.RemoteSub()
	items, err := c.memcConn.GetMulti([]string{localkey, remotekey})
	if err != nil && err != memcache.ErrCacheMiss {
		return nil, err
	}
	var allsubs []*serviceinstance
	if len(items[localkey].Value) != 0 {
		localsubs := bytes.Split(items[localkey].Value, []byte{47})
		allsubs = make([]*serviceinstance, 0, len(localsubs))
		for _, subname := range localsubs {
			if subinstance, ok := c.localservices[ServiceName(subname)]; ok {
				allsubs = append(allsubs, subinstance)
			} else {
				l.Warning("getAllSubs", suckutils.ConcatThree("serviceinstance \"", string(subname), "\" not represented in configurator's data"))
			}
		}
	}
	return allsubs, nil

	//allsubscriptors := make([]serviceinstance, 0, len(localsubs)+(len(items[servicename.RemoteSub()].Value)/4))
}

func getAllServiceAddresses(memcConn *memcache.Client, servicename ServiceName) ([]byte, error) {
	if servicename == "" {
		return nil, errors.New("servicename must not be empty")
	}
	keys := []string{servicename.Local(), servicename.Remote()}
	items, err := memcConn.GetMulti(keys)
	if err != nil {
		return nil, err
	}
	addresses := make([]byte, 0)
	for _, item := range items {
		if item.Key == keys[0] {
			if len(item.Value) < 7 {
				if len(item.Value) != 0 {
					// TODO: ?
				}
				continue
			}
			addresses = append(addresses, item.Value[:6]...)
		} else {
			if len(item.Value) < 6 {
				continue
			}
			for i := 0; i < len(item.Value)/6; i++ {
				addresses = append(addresses, item.Value[i*6:i*6+6]...)
			}
		}
	}
	return addresses, nil
}

// ONLY FOR LOCAL SERVICES, ONLY MEMC
func (c *Configurator) updateMemcServiceStatus(l *logscontainer.WrappedLogsContainer, servicename ServiceName, newstatus ServiceStatus) error {
	item, err := c.memcConn.Get(servicename.Local()) // TODO: может лучше вытаскивать из мапы сразу?
	if err != nil && err != memcache.ErrCacheMiss {
		return err
	}
	if len(item.Value) < 7 {
		item := &memcache.Item{}
		if _, ok := c.localservices[servicename]; ok { // возможна ли такая ситуация? если только после падения-поднятия мемкэша?
			item.Value = c.localservices[servicename].addr.WithStatus(newstatus)
			l.Warning("updateStatus", "\"local.\" cache miss, restored from configurator's data")
		} else {
			return errors.New("unknown service")
		}
		item.Key = servicename.Local()
	} else {
		item.Value = IPv4withPort(item.Value).WithStatus(newstatus)
	}
	return c.memcConn.Set(item)
}

func getfreeaddr() (IPv4withPort, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, err
	}
	l.Close()
	return ParseIPv4withPort(addr.String()), nil
}

func SendMessage(conn net.Conn, sideState ws.State, operationCode OperationCode, message []byte) error {
	if conn == nil {
		return errors.New("conn is nil")
	}
	payload := make([]byte, len(message)+1)
	payload = append(append(payload, byte(operationCode)), message...)

	if sideState.ServerSide() {
		return ws.WriteFrame(conn, ws.NewBinaryFrame(payload))
	} else if sideState.ClientSide() {
		return ws.WriteFrame(conn, ws.MaskFrameInPlace(ws.NewBinaryFrame(payload)))
	}
	return errors.New("unknown sideState")
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
