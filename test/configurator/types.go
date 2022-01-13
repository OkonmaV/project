package main

import (
	"errors"
	"net"
	"project/test/types"
	"strconv"
	"strings"
	"time"

	"github.com/big-larry/suckutils"
)

const ReconnectEnabledFlag string = "-reconnect"

type ServiceName string

// TODO: при подрубе удаленных сервисов 100% будут проблемы, поэтому так пока нельзя делать
// TODO: рассмотреть возможность дифференцировать адрес и порт, навскидку непонятно нужно ли
type Address struct {
	addr   string
	netw   types.NetProtocol
	random bool
}

func readAddress(rawline string) *Address {
	sep_ind := strings.Index(rawline, ":")
	if sep_ind == -1 {
		return nil
	}
	addr := &Address{addr: (rawline)[sep_ind+1:]}

	switch (rawline)[:sep_ind] {
	case "tcp":
		addr.netw = types.NetProtocolTcp
	case "unix":
		addr.netw = types.NetProtocolUnix
	case "nil":
		addr.netw = types.NetProtocolNil
	default:
		return nil
	}
	if addr.addr == "*" {
		addr.random = true
		return addr
	}
	if !addr.netw.Verify(addr.addr) {
		return nil
	}
	return addr
}

func (a *Address) getListeningAddr() (types.NetProtocol, string, error) {
	if a == nil {
		return 0, "", errors.New("nil address struct")
	}
	if a.random {
		var err error
		for i := 0; i < 3; i++ {
			addr, err := getfreeaddr(a.netw)
			if err != nil {
				continue
			}
			return a.netw, addr, nil
		}
		return 0, "", err
	}
	return a.netw, a.addr, nil
}

// если адреса рандомны то всегда true (хз как назвать очевиднее)
func (addr1 *Address) equalAsListenAddr(addr2 Address) bool {
	if addr1.random == addr2.random {
		if addr1.random {
			return true
		}
		return addr1.netw == addr2.netw && addr1.addr == addr2.addr
	}
	return false
}

func getfreeaddr(netw types.NetProtocol) (string, error) {
	switch netw {
	case types.NetProtocolTcp:
		addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		if err != nil {
			return "", err
		}
		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return "", err
		}
		l.Close()
		return addr.String(), nil
	case types.NetProtocolUnix:
		return suckutils.Concat("/tmp/", strconv.FormatInt(time.Now().UnixNano(), 10), ".sock"), nil
	}
	return "", errors.New("unknown protocol")
}

// DOES NOT FORMAT THE MESSAGE
func sendToMany(message []byte, recievers []*service) {
	if len(recievers) == 0 {
		return
	}
	for _, reciever := range recievers {
		if reciever.connector.IsClosed() {
			continue
		}
		if err := reciever.connector.Send(message); err != nil {
			reciever.connector.Close(err)
		}
	}
}
