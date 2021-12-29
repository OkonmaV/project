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

const ReconnectEnabledFlag string = "-rec"

type ServiceName string

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

func (a *Address) getListeningAddr() (types.NetProtocol, string) {
	if a.random {
		addr, err := getfreeaddr(a.netw)
		if err != nil {
			return 0, ""
		}
		return a.netw, addr
	}
	return a.netw, a.addr
}

func (addr1 *Address) equal(addr2 *Address) bool {
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
