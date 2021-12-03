package client

import (
	"encoding/binary"
	"errors"
	"net"
	"project/test/connector"
	"strings"
	"time"

	"github.com/big-larry/suckutils"
)

func (ci *pubConnectorinfo) handlepub(payload []byte) error {
	return nil
}

func (ci *pubConnectorinfo) handlepubclose(reason error) {
	return
}

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
				s.OnAirMux.Lock()
				if s.OnAir {
					s.OnAir = false
					s.OnAirMux.Unlock()
					if err := s.Sendtoconf([]byte{byte(OperationCodeSetMyStatusSuspended)}); err != nil {
						//s.confcon.Close(err) // TODO: обсуждаемо
						l.Error("Conf", errors.New(suckutils.ConcatTwo("cant send new status suspend, err: ", err.Error())))
					}
				} else {
					s.OnAirMux.Unlock()
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
					go pubconinfo.reconnectToPub()
				} else {
					s.pubsrwmux.Lock()
					if cons, ok := s.pubs[name]; ok {
						for k := 0; k < len(cons); k++ {
							if cons[k].address == addr {
								close(cons[k].closech)
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
func (p *pubConnectorinfo) reconnectToPub() (notcontinue bool) {
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
			return true
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
