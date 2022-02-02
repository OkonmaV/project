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

// format: "-remote:(host addr)", e.g. "-remote:195.21.65.87"
const RemoteEnabledFlag string = "-remote:"

type ServiceName string

// TODO: при подрубе удаленных сервисов 100% будут проблемы, поэтому так пока нельзя делать
// TODO: рассмотреть возможность дифференцировать адрес и порт, навскидку непонятно нужно ли
type Address struct {
	netw       types.NetProtocol
	remotehost string
	port       string
	random     bool
}

func readAddress(rawline string) *Address {
	sep_ind := strings.Index(rawline, ":")
	if sep_ind == -1 {
		return nil
	}
	addr := &Address{port: (rawline)[sep_ind+1:]}
	if addr.port == "*" {
		addr.random = true
	}

	switch (rawline)[:sep_ind] {
	case "tcp":
		addr.netw = types.NetProtocolTcp
		if _, err := strconv.ParseUint(addr.port, 10, 16); err != nil {
			return nil
		}
	case "unix":
		addr.netw = types.NetProtocolUnix
		if !addr.netw.Verify(addr.port) {
			return nil
		}
	case "nil":
		addr.netw = types.NetProtocolNil
		if !addr.netw.Verify(addr.port) {
			return nil
		}
	default:
		return nil
	}

	return addr
}

func (a *Address) getListeningAddr() (types.NetProtocol, string, error) {
	if a == nil {
		return 0, "", errors.New("nil address struct")
	}
	if a.random {
		if len(a.remotehost) != 0 {
			return 0, "", errors.New("cant get random tcp-listening-addr for remote host")
		}

		if a.port != "*" {
			var addr string
			if a.netw == types.NetProtocolTcp {
				addr = suckutils.ConcatTwo("127.0.0.1:", a.port)
			} else {
				addr = a.port
			}
			if l, err := net.Listen(a.netw.String(), addr); err == nil {
				l.Close()
				return a.netw, addr, nil
			}
		}

		var err error
		for i := 0; i < 3; i++ {
			a.port, err = getFreePort(a.netw)
			if err != nil {
				continue
			}
			return a.netw, suckutils.ConcatTwo("127.0.0.1:", a.port), nil
		}
		return 0, "", err
	}
	return a.netw, suckutils.ConcatTwo("127.0.0.1:", a.port), nil
}

// если адреса рандомны то всегда true
func (addr1 *Address) equalAsListeningAddr(addr2 Address) bool {
	if addr1.random == addr2.random {
		if addr1.random {
			return true
		}
		return addr1.netw == addr2.netw && addr1.remotehost == addr2.remotehost && addr1.port == addr2.port
	}
	return false
}

func getFreePort(netw types.NetProtocol) (string, error) {
	switch netw {
	case types.NetProtocolTcp:
		addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		if err != nil {
			return "", err
		}

		return strconv.Itoa(addr.Port), nil
	case types.NetProtocolUnix:
		return suckutils.Concat("/tmp/", strconv.FormatInt(time.Now().UnixNano(), 10), ".sock"), nil
	case types.NetProtocolNil:
		return "", nil
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
