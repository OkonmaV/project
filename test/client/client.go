package client

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"os/signal"
	"project/test/connector"
	"strings"
	"sync"
	"time"

	"github.com/big-larry/suckutils"
)

type Service struct {
	name      ServiceName
	pubs      map[ServiceName][]*pubConnectorinfo // TODO: рассмотреть возможность переделать в массив
	pubsrwmux sync.RWMutex
	//pubupdate chan<- pubsettingsupdate
	//instancesFeedback []chan []byte
	handler  ClientHandleSub
	onair    bool
	onairmux sync.RWMutex
	confcon  *connector.Connector
}

type pubConnectorinfo struct {
	address string
	network string
	//isclosed func() bool
	servicename ServiceName
	l           logger
	//handle      func([]byte) error
	send    func([]byte) error
	closech chan struct{} // чтобы остановить реконнект
}

type Connectorinfo struct {
	servicename   ServiceName
	l             logger
	getremoteaddr ClientGetConAddr
	handle        ClientHandleSub
	send          func([]byte) error
}

type logger interface {
	Debug(string, string)
	Info(string, string)
	Warning(string, string)
	Error(string, error)
}

type ClientPubResponce chan []byte
type ClientDoOnPubDisconnect func()
type ClientHandleSub func(logger, ServiceName, ClientGetConAddr, ClientSendResponce, []byte) ([]byte, error) // TODO: возврат ошибки вызовет закрытие коннекшна, обсудить
type ClientGetConAddr func() (string, string)
type ClientSendResponce func([]byte) error

func (ci *pubConnectorinfo) handlepub(payload []byte) error {
	return nil
}

func (ci *pubConnectorinfo) handlepubclose(reason error) {
	return
}

func (ci *Connectorinfo) handlesub(payload []byte) error {
	if len(payload) < 2 {
		return connector.ErrWeirdData
	}
	responce, err := ci.handle(ci.l, ci.servicename, ci.getremoteaddr, nil, payload[2:])
	if err != nil {
		return err // TODO: вызовет закрытие коннекшна, обсудить
	}
	respbuf := make([]byte, 0, 2+len(responce))
	if err = ci.send(append(append(respbuf, payload[:2]...), responce...)); err != nil {
		ci.l.Error("Send", err)
	}
	return err
}

func (ci *Connectorinfo) handlesubclose(err error) {
	_, addr := ci.getremoteaddr()
	ci.l.Debug("Connector", suckutils.Concat("con with ", string(ci.servicename), " from ", addr, " closed, reason: ", err.Error()))
	return
}

//type ClientSendToSub func([]byte, []byte) error

//TODO: сделать каналы для фидбэка или аналог
func NewService(l logger, servicename ServiceName, confaddress string, instancesnum int, subshandler ClientHandleSub, pubnames ...ServiceName) (*Service, error) {
	if instancesnum < 1 {
		return nil, errors.New("instancesnum must be greater than 0")
	}
	if len(servicename) == 0 {
		return nil, errors.New("empty servicename")
	}
	connector.SetupGopoolHandling(instancesnum, 1, instancesnum)

	service := &Service{}
	confconinfo := &pubConnectorinfo{network: (confaddress)[:strings.Index(confaddress, ":")], address: (confaddress)[strings.Index(confaddress, ":")+1:], servicename: ConfServiceName, l: l}
	confconn, err := net.Dial(confconinfo.network, confconinfo.address)
	if err != nil {
		return nil, err
	}

	if service.confcon, err = connector.NewConnector(confconn, confconinfo.handlepub, confconinfo.handlepubclose); err != nil {
		return nil, err
	}
	confconinfo.send = service.confcon.Send
	if err = service.confcon.StartServing(); err != nil {
		return nil, err
	}
	if err = confconinfo.send([]byte(servicename)); err != nil {
		return nil, err
	}
}

// type pubsettingsupdate struct {
// 	pubname   ServiceName
// 	network   string
// 	address   string
// 	newstatus bool
// }

func (s *Service) servepubs(l logger, ch <-chan []byte) {
	ticker := time.NewTicker(time.Second * 5)
	// defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			var makesuspended bool
			for name, cons := range s.pubs {
				if len(cons) == 0 {
					makesuspended = true
					l.Warning("Pub", suckutils.ConcatTwo("no active connections to pub ", string(name)))
				}
			}
			if makesuspended {
				s.onairmux.Lock()
				if s.onair {
					s.onair = false
					s.onairmux.Unlock()
					if err := s.confcon.Send([]byte{byte(OperationCodeSetMyStatusSuspended)}); err != nil {
						s.confcon.Close(err) // TODO: обсуждаемо
					}
				} else {
					s.onairmux.Unlock()
				}
			}
		case update := <-ch: // TODO: перенести разбор массива в хэндлер на случай кривых данных
			for i := 0; i < len(update); {
				if len(update) < i+2 {
					l.Error("Servepubs", errors.New("unreadable update"))
					break
				}
				namelen := int(binary.BigEndian.Uint16(update[i : i+2]))
				i += 2
				if len(update) < i+namelen {
					l.Error("Servepubs", errors.New("unreadable update"))
					break
				}
				name := ServiceName(update[i : i+namelen])
				i += namelen
				if len(update) < i+2 {
					l.Error("Servepubs", errors.New("unreadable update"))
					break
				}
				addrlen := int(binary.BigEndian.Uint16(update[i : i+2]))
				i += 2
				if len(update) < i+addrlen+1 { // +1 чтобы сразу новый статус захватить
					l.Error("Servepubs", errors.New("unreadable update"))
					break
				}
				rawaddr, netw, addr := string(update[i:i+addrlen]), "", ""
				if ind := strings.Index(rawaddr, ":"); ind > 0 {
					netw = rawaddr[:ind]
					addr = rawaddr[ind+1:]
				} else {
					l.Error("Servepubs", errors.New("unreadable update"))
					break
				}
				i += addrlen
				if update[i] == byte(StatusOn) {
					pubconinfo := &pubConnectorinfo{network: netw, address: addr, servicename: name, l: l, closech: make(chan struct{})}
					go pubconinfo.reconnect()
				} else {
					s.pubsrwmux.Lock()
					if cons, ok := s.pubs[name]; ok {
						for k := 0; k < len(cons); k++ {
							if cons[k].address == addr {
								cons = append(cons[:k], cons[k+1:]...)
								l.Debug("Servepubs", suckutils.Concat("new pub \"", string(name), "\" from ", addr, " added"))
								break
							}
						}
					} else {
						l.Error("Servepubs", errors.New(suckutils.ConcatThree("unknown pub \"", string(name), "\" in update")))
					}
					s.pubsrwmux.Unlock()
				}
				i++
			}
		}
	}
}

// передаем слайс и мьютекс при нужде, чтобы в пабах не висел отвалившийся коннектор ()
// TODO: подумать и сделать из реконнекта просто коннект с опцией реконнекта
func (p *pubConnectorinfo) reconnect(pubcons []*pubConnectorinfo, pubmux sync.RWMutex) {
	conn, err := net.Dial(p.network, p.address)
	if err != nil {
		p.l.Error("Reconnect", err)
		err = nil
	} else {
		con, err := connector.NewConnector(conn, p.handlepub, p.handlepubclose)
		if err != nil {
			p.l.Error("Reconnect|NewConnector", err)
			err = nil
		} else {
			p.send = con.Send
			if err = con.StartServing(); err != nil {
				p.l.Error("Reconnect|StartServing", err)
				err = nil
			} else {
				p.l.Debug("Reconnect", suckutils.ConcatFour(string(p.servicename), " from ", p.address, " reconnected"))
				if pubcons != nil {
					pubmux.Lock()
					pubcons = append(pubcons, p)
					pubmux.Unlock()
				}
				return
			}
		}
	}
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	for {
		select {
		case <-p.closech:
			p.l.Debug("Reconnect", suckutils.Concat("to ", string(p.servicename), " from ", p.address, " stopped (settings changed)"))
			return
		case <-ticker.C:
			con, err := connector.NewConnector(conn, p.handlepub, p.handlepubclose)
			if err != nil {
				p.l.Error("Reconnect|NewConnector", err)
				err = nil
			} else {
				p.send = con.Send
				if err = con.StartServing(); err != nil {
					p.l.Error("Reconnect|StartServing", err)
					err = nil
				} else {
					p.l.Debug("Reconnect", suckutils.ConcatFour(string(p.servicename), " from ", p.address, " reconnected"))
					return
				}
			}
		}
	}
}

type ClientWaitResponce func() ([]byte, error)
type respch chan []byte

// TODO: чо с каналами?
func (ch respch) waitResponceFromPub() ([]byte, error) {
	timer := time.NewTimer(time.Second * 2)
	select {
	case payload := <-ch:
		timer.Stop()
		if payload == nil {
			return nil, errors.New("nil responce")
		}
		return payload, nil
	case <-timer.C:
		return nil, errors.New("no responce on deadline")
	}
}

// TODO: вот те {5,6} в ретурне порешить
func (s *Service) SendToPub(pubname ServiceName, payload []byte) (ClientWaitResponce, error) {
	buf := make([]byte, 0, 2+len(payload))
	if cons, ok := s.pubs[pubname]; ok {
		ch := make(respch, 1)
		return ch.waitResponceFromPub, cons[0].send(append(append(buf, []byte{5, 6}...), payload...))
	} else {
		return nil, errors.New("no pubs with that name")
	}
}

func createContextWithInterruptSignal() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		<-stop
		cancel()
	}()
	return ctx, cancel
}
