package main

import (
	"project/test/types"
	"strings"
)

type ServiceName string

type Address struct {
	addr string // port or socket-path
	netw types.NetProtocol
}

func readAddress(rawline string) *Address {
	sep_ind := strings.Index(rawline, ":")
	addr := &Address{addr: (rawline)[sep_ind+1:]}
	switch (rawline)[:sep_ind] {
	case "tcp":
		addr.netw = types.NetProtocolTcp
	case "unix":
		addr.netw = types.NetProtocolUnix
	default:
		return nil
	}
	if !addr.netw.Check(addr.addr) {
		addr = nil
	}
	return addr
}
