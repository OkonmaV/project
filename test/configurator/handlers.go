package main

import (
	"errors"
	"project/test/connector"
	"project/test/types"
	"strings"

	"github.com/big-larry/suckutils"
)

func (s *service) NewMessage() connector.MessageReader {
	return connector.NewBasicMessage()
}

func (s *service) Handle(message connector.MessageReader) error {
	if !s.ownStatus.OnAir() {
		s.l.Debug("onAir", "suspended, dont handle the message")
		return s.connector.Send(connector.FormatBasicMessage([]byte{byte(types.OperationCodeImSupended)}))
	}

	payload := message.(connector.BasicMessage).Payload
	if len(payload) == 0 {
		return connector.ErrEmptyPayload
	}
	switch types.OperationCode(payload[0]) {
	//case types.OperationCodeImSupended: // TODO
	case types.OperationCodePing:
		return nil
	case types.OperationCodeGiveMeOuterAddr:
		if netw, addr, err := s.outerAddr.getListeningAddr(); err != nil {
			return errors.New(suckutils.ConcatTwo("getlisteningaddr err: ", err.Error()))
		} else {
			formatted_addr := types.FormatAddress(netw, addr)
			if err := s.connector.Send(connector.FormatBasicMessage(append(append(make([]byte, 0, len(formatted_addr)+2), byte(types.OperationCodeSetOutsideAddr), byte(len(formatted_addr))), formatted_addr...))); err != nil {
				return err
			}
			return nil
		}
	case types.OperationCodeSubscribeToServices:
		raw_pubnames := types.SeparatePayload(payload[1:])
		if raw_pubnames == nil {
			return connector.ErrWeirdData
		}
		pubnames := make([]ServiceName, 0, len(raw_pubnames))
		for _, raw_pubname := range raw_pubnames {
			if len(raw_pubname) == 0 {
				return connector.ErrWeirdData
			}
			pubnames = append(pubnames, ServiceName(raw_pubname))
		}
		return s.subs.subscribe(s, pubnames...)

	case types.OperationCodeUpdatePubs:
		if s.name == ServiceName(types.ConfServiceName) {
			updates := types.SeparatePayload(payload[1:])
			if len(updates) != 0 {
				foo := s.connector.RemoteAddr().String()
				external_ip := (foo)[:strings.Index(foo, ":")]
				for _, update := range updates {
					pubname, raw_addr, status, err := types.UnformatOpcodeUpdatePubMessage(update)
					if err != nil {
						s.l.Error("UnformatOpcodeUpdatePubMessage", err)
						return connector.ErrWeirdData
					}
					netw, addr, err := types.UnformatAddress(raw_addr)
					if err != nil {
						s.l.Error("UnformatAddress", err)
						return connector.ErrWeirdData
					}
					switch netw {
					case types.NetProtocolUnix:
						netw = types.NetProtocolNonlocalUnix
					case types.NetProtocolTcp:
						if (addr)[:strings.Index(addr, ":")] == "127.0.0.1" {
							addr = suckutils.ConcatTwo(external_ip, (addr)[strings.Index(addr, ":"):])
						}
					}
					s.subs.updatePub(pubname, types.FormatAddress(netw, addr), status, false)
				}
			}
		} else {
			return errors.New("not configurator, but sent OperationCodeUpdatePubs")
		}
	case types.OperationCodeMyStatusChanged:
		if len(payload) < 2 {
			return connector.ErrWeirdData
		}
		s.changeStatus(types.ServiceStatus(payload[1]))
	default:
		return connector.ErrWeirdData
	}
	return nil
}

func (s *service) HandleClose(reason error) {
	s.l.Warning("Conn", suckutils.ConcatTwo("closed, reason err: ", reason.Error()))
	s.changeStatus(types.StatusOff)
}
