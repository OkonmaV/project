package main

import (
	"errors"
	"net"
	"project/test/connector"
	"project/test/defaultlogger"

	"github.com/big-larry/suckutils"
	"github.com/bradfitz/gomemcache/memcache"
)

type connectordata struct {
	servicename  ServiceName
	outsideaddr  Port
	islocalhost  bool
	configurator *Configurator
}

func (cd *connectordata) Getlogger() defaultlogger.DefaultLogger {
	return l
}

func (cd *connectordata) HandleDisconnect(con *connector.Connector) {
	l.Warning(string(cd.servicename), "disconnected")
	_, remaddr := con.GetRemoteAddr()
	if cd.islocalhost {
		cd.updateLocalServiceStatus(remaddr, StatusOff)
	} else {
		//reconnect to remote.conf
	}
}

func (cd *connectordata) Handle(con *connector.Connector, opcode connector.OperationCode, payload []byte) error {
	switch opcode {
	case connector.OperationCodeSetMyStatusOn:
		_, remaddr := con.GetRemoteAddr()
		if cd.islocalhost {
			cd.updateLocalServiceStatus(remaddr, StatusOn)
		} else {
			// TODO:
		}
	case connector.OperationCodeSetMyStatusSuspended:
		_, remaddr := con.GetRemoteAddr()
		if cd.islocalhost {
			cd.updateLocalServiceStatus(remaddr, StatusSuspended)
		} else {
			// TODO:
		}
	case connector.OperationCodeSubscribeToServices:
		pubs := separatePayload(payload)
		if len(pubs) == 0 {
			l.Warning("OperationCodeSubscribeToServices", "err when separating payload")
			con.Close()
			return connector.ErrNotResume
		}
		pubnames := make([]ServiceName, 0, len(pubs))
		for _, pubname := range pubs {
			pubnames = append(pubnames, ServiceName(pubname))
		}
		cd.configurator.subscribeToServices(cd.servicename, con, pubnames)
	case connector.OperationCodeSetPubAddresses:
		if cd.servicename != ConfServiceName || !cd.islocalhost {
			con.Close()
			l.Warning("OperationCodeSetPubAddresses", "not remote conf")
			return connector.ErrNotResume
		}
		if len(payload) == 0 {
			l.Warning("OperationCodeSetPubAddresses", "empty payload")
			return nil
		}
		servicenamelength := int(payload[0])
		if servicenamelength < len(payload) {
			servicename := ServiceName(payload[1 : servicenamelength+1])
			serviceaddrs := make([]IPv4withPort, 0, (len(payload)-servicenamelength-1)/6)
			for i := servicenamelength + 7; i < len(payload); i = +6 {
				addr := ParseIPv4withPort(string(payload[i-6 : i]))
				if addr != nil {
					serviceaddrs = append(serviceaddrs, addr)
				} else {
					l.Warning("OperationCodeSetPubAddresses", "cant parse addr")
				}
			}
		}

	}
	l.Info("PAYLOAD", string(payload)) // TODO: DELETE THIS <----------------------------------
}

// TODO: логика первой отправки адресов пабов не написана
func (c *Configurator) subscribeToServices(subname ServiceName, subconnector *connector.Connector, pubnames []ServiceName) error {
	if subname == "" || len(pubnames) == 0 {
		return errors.New("servicenames params must not be nil")
	}
	c.subscriptions.rwmux.RLock()
	for _, pubname := range pubnames {
		if subs, ok := c.subscriptions.connectors[pubname]; ok {
			var subscribed bool
			for i := 0; i < len(subs); i++ {
				if subconnector == subs[i] {
					l.Warning(string(subname), suckutils.ConcatThree("try to subscribe to ", string(pubname), " when already subscribed"))
					subscribed = true
					break
				}
			}
			if !subscribed {
				c.subscriptions.connectors[pubname] = append(c.subscriptions.connectors[pubname], subconnector)
				l.Debug(string(subname), suckutils.ConcatTwo("subscribed to ", string(pubname)))
			}
		} else {
			c.subscriptions.connectors[pubname] = make([]*connector.Connector, 0, 1)
			c.subscriptions.connectors[pubname] = append(c.subscriptions.connectors[pubname], subconnector)
			l.Debug(string(subname), suckutils.ConcatTwo("subscribed to unknown ", string(pubname)))
		}
	}
	c.subscriptions.rwmux.RUnlock()
	return nil
}

func (cd *connectordata) updateLocalServiceStatus(connremoteaddr string, newstatus ServiceStatus) error {
	if connremoteaddr == "" {
		return errors.New("connremoteaddr is empty")
	}
	var payload []byte
	if cd.outsideaddr != nil { // значит торчит наружу
		cd.configurator.localservices.rwmux.Lock()
		cd.configurator.localservices.serviceinfo[cd.servicename][cd.outsideaddr.String()] = newstatus
		cd.configurator.localservices.rwmux.Unlock()
		payload = make(IPv4withPort, 6).WithStatus(newstatus)
	} else {
		payload = cd.outsideaddr.NewHost(IPv4{127, 0, 0, 1}).WithStatus(newstatus)
	}
	cd.configurator.localservicesstatuses.rwmux.Lock()
	cd.configurator.localservicesstatuses.serviceinfo[cd.servicename][connremoteaddr] = newstatus
	cd.configurator.localservicesstatuses.rwmux.Unlock()

	cd.configurator.subscriptions.rwmux.RLock()
	subs := cd.configurator.subscriptions.connectors[cd.servicename]
	cd.configurator.subscriptions.rwmux.RUnlock()
	for _, con := range subs {
		if err := con.Send(connector.OperationCodeUpdatePubStatus, payload); err != nil {
			l.Error(suckutils.ConcatTwo("Send to ", string(cd.servicename)), err)
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
		if err := con.Send(connector.OperationCodeUpdatePubStatus, serviceaddr.WithStatus(newstatus)); err != nil {
			l.Error(suckutils.ConcatTwo("Send to ", string(servicename)), err)
		}
	}
	return nil
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
