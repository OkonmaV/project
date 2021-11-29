package main

import (
	"errors"
	"net"
	"project/test/connector"

	"github.com/big-larry/suckutils"
	"github.com/bradfitz/gomemcache/memcache"
)

type connectorinfo struct {
	servicename   ServiceName
	outsideaddr   Port
	islocal       bool
	configurator  *Configurator
	getremoteaddr func() (string, string)
	isclosedcon   func() bool
	send          func([]byte) error
	subscribe     func([]ServiceName) error
}

func (ci *connectorinfo) HandleClose(reasonerr error) {
	l.Warning(string(ci.servicename), suckutils.ConcatTwo("disconnected, reason err: ", reasonerr.Error()))
	_, remaddr := ci.getremoteaddr()
	if ci.islocal {
		ci.updateLocalServiceStatus(remaddr, StatusOff)
	} else {
		//reconnect to remote.conf
	}
}

func (ci *connectorinfo) Handle(payload []byte) error {
	var opcode OperationCode
	if len(payload) > 0 {
		opcode = OperationCode(payload[0])
	} else {
		return connector.ErrWeirdData
	}
	switch opcode {
	case OperationCodeSetMyStatusOn:
		_, remaddr := ci.getremoteaddr()
		if ci.islocal {
			ci.updateLocalServiceStatus(remaddr, StatusOn)
		} else {
			// TODO:
		}
	case OperationCodeSetMyStatusSuspended:
		_, remaddr := ci.getremoteaddr()
		if ci.islocal {
			ci.updateLocalServiceStatus(remaddr, StatusSuspended)
		} else {
			// TODO:
		}
	case OperationCodeSubscribeToServices:
		pubs := separatePayload(payload)
		if len(pubs) == 0 {
			l.Warning("OperationCodeSubscribeToServices", "err when separating payload")
			return connector.ErrWeirdData
		}
		pubnames := make([]ServiceName, 0, len(pubs))
		for _, pubname := range pubs {
			pubnames = append(pubnames, ServiceName(pubname))
		}
		if err := ci.subscribe(pubnames); err != nil {
			l.Error("subscribe", err)
			return connector.ErrWeirdData
		}
		// case OperationCodeSetPubAddresses:
		// 	if ci.servicename != ConfServiceName || !ci.islocal {
		// 		l.Warning("OperationCodeSetPubAddresses", "not remote conf")
		// 		return connector.ErrWeirdData
		// 	}
		// 	if len(payload) == 0 {
		// 		l.Warning("OperationCodeSetPubAddresses", "empty payload")
		// 		return nil
		// 	}
		// 	servicenamelength := int(payload[0])
		// 	if servicenamelength < len(payload) {
		// 		servicename := ServiceName(payload[1 : servicenamelength+1])
		// 		serviceaddrs := make([]IPv4withPort, 0, (len(payload)-servicenamelength-1)/6)
		// 		for i := servicenamelength + 7; i < len(payload); i = +6 {
		// 			addr := ParseIPv4withPort(string(payload[i-6 : i]))
		// 			if addr != nil {
		// 				serviceaddrs = append(serviceaddrs, addr)
		// 			} else {
		// 				l.Warning("OperationCodeSetPubAddresses", "cant parse addr")
		// 			}
		// 		}
		// 	}
	}
	return nil
}

// TODO: вынести логгирование из функции
// TODO: логика первой отправки адресов пабов не написана
func (ci *connectorinfo) subscribeToServices(subcon *connector.Connector, pubnames []ServiceName) error {
	if len(pubnames) == 0 {
		return errors.New("servicenames params must not be nil")
	}
	ci.configurator.subscriptions.rwmux.RLock()
	for _, pubname := range pubnames {
		if subs, ok := ci.configurator.subscriptions.connectors[pubname]; ok {
			var subscribed bool
			for i := 0; i < len(subs); i++ {
				if subcon == subs[i] {
					l.Warning(string(ci.servicename), suckutils.ConcatThree("try to subscribe to ", string(pubname), " when already subscribed"))
					subscribed = true
					break
				}
			}
			if !subscribed {
				ci.configurator.subscriptions.connectors[pubname] = append(ci.configurator.subscriptions.connectors[pubname], subcon)
				l.Debug(string(ci.servicename), suckutils.ConcatTwo("subscribed to ", string(pubname)))
			}
		} else {
			ci.configurator.subscriptions.connectors[pubname] = make([]*connector.Connector, 0, 1)
			ci.configurator.subscriptions.connectors[pubname] = append(ci.configurator.subscriptions.connectors[pubname], subcon)
			l.Debug(string(ci.servicename), suckutils.ConcatTwo("subscribed to unknown ", string(pubname)))
		}
	}
	ci.configurator.subscriptions.rwmux.RUnlock()
	return nil
}

func (ci *connectorinfo) updateLocalServiceStatus(connremoteaddr string, newstatus ServiceStatus) error {
	if connremoteaddr == "" {
		return errors.New("connremoteaddr is empty")
	}
	var payload []byte
	if ci.outsideaddr != nil { // значит торчит наружу
		ci.configurator.localservices.rwmux.Lock()
		ci.configurator.localservices.serviceinfo[ci.servicename][ci.outsideaddr.String()] = newstatus
		ci.configurator.localservices.rwmux.Unlock()
		payload = make(IPv4withPort, 6).WithStatus(newstatus)
	} else {
		payload = ci.outsideaddr.NewHost(IPv4{127, 0, 0, 1}).WithStatus(newstatus)
	}
	ci.configurator.localservicesstatuses.rwmux.Lock()
	ci.configurator.localservicesstatuses.serviceinfo[ci.servicename][connremoteaddr] = newstatus
	ci.configurator.localservicesstatuses.rwmux.Unlock()

	ci.configurator.subscriptions.rwmux.RLock()
	subs := ci.configurator.subscriptions.connectors[ci.servicename]
	ci.configurator.subscriptions.rwmux.RUnlock()
	for _, con := range subs {
		if err := con.Send(payloadWithOpCode(OperationCodeSetPubAddresses, payload)); err != nil {
			l.Error(suckutils.ConcatTwo("Send to ", string(ci.servicename)), err)
		}
	}
	return nil
}

func (c *Configurator) updateRemoteServiceStatus(servicename ServiceName, serviceaddr IPv4withPort, newstatus ServiceStatus) error {
	if servicename == "" || len(serviceaddr) != 6 {
		return errors.New("wrong/empty arguments")
	}
	c.subscriptions.rwmux.RLock()
	subs := c.subscriptions.connectors[servicename]
	c.subscriptions.rwmux.RUnlock()
	c.remoteservices.rwmux.Lock()
	c.remoteservices.serviceinfo[servicename][serviceaddr.String()] = newstatus
	c.remoteservices.rwmux.Unlock()
	for _, con := range subs {
		if err := con.Send(payloadWithOpCode(OperationCodeUpdatePubStatus, serviceaddr.WithStatus(newstatus))); err != nil {
			l.Error(suckutils.ConcatTwo("Send to ", string(servicename)), err)
		}
	}
	return nil
}

func payloadWithOpCode(opcode OperationCode, payload []byte) []byte {
	newpayload := make([]byte, 0, len(payload)+1)
	return append(append(newpayload, byte(OperationCodeUpdatePubStatus)), payload...)
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

func separatePayload(payload []byte) [][]byte {
	if len(payload) == 0 {
		return nil
	}
	items := make([][]byte, 0, 1)
	for i := 0; i < len(payload); {
		length := int(payload[i])
		if i+int(length)+1 > len(payload) {
			return nil
		}
		items = append(items, payload[i+1:i+length+1])
		i = length + 1 + i
	}
	return items
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
