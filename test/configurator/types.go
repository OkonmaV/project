package main

import (
	"encoding/binary"
	"net"
	"strconv"
	"strings"

	"github.com/big-larry/suckutils"
)

type serviceinfo struct {
	name     ServiceName
	isRemote bool
}

func (servinfo *serviceinfo) nameWithLocationType() string {
	if servinfo.isRemote {
		return servinfo.name.Remote()
	}
	return servinfo.name.Local()
}

type ServiceStatus byte

const (
	StatusOff       ServiceStatus = 0
	StatusOn        ServiceStatus = 1
	StatusSuspended ServiceStatus = 2
)

func (status ServiceStatus) String() string {
	switch status {
	case StatusOff:
		return "Off"
	case StatusOn:
		return "On"
	case StatusSuspended:
		return "Suspended"
	default:
		return "Undefined"
	}
}

type ServiceName string

const ConfServiceName ServiceName = "conf"

func (sn ServiceName) Local() string {
	return suckutils.ConcatTwo("local.", string(sn))
}

func (sn ServiceName) LocalSub() string {
	return suckutils.ConcatTwo("local.sub.", string(sn))
}

func (sn ServiceName) Remote() string {
	return suckutils.ConcatTwo("remote.", string(sn))
}

func (sn ServiceName) RemoteSub() string {
	return suckutils.ConcatTwo("remote.sub", string(sn))
}

type Addr []byte

var AddrByteOrder = binary.LittleEndian

func ParseIPv4withPort(addr string) Addr {
	foo := strings.Split(addr, ":")
	if len(foo) != 2 {
		return nil
	}
	address := make([]byte, 0, 6)
	address = append(address, net.ParseIP(foo[0]).To4()...)
	if len(address) == 0 {
		return nil
	}
	address = append(address, []byte{0, 0}...)
	port, err := strconv.ParseUint(foo[1], 10, 16)
	if err != nil {
		return nil
	}
	AddrByteOrder.PutUint16(address[4:], uint16(port))
	return address
}

// with or without port, else return empty string
func (address Addr) String() string {
	if len(address) < 6 {
		if len(address) == 4 {
			return net.IPv4(address[0], address[1], address[2], address[3]).String()
		}
		return ""
	}
	return suckutils.ConcatThree(net.IPv4(address[0], address[1], address[2], address[3]).String(), ":", strconv.Itoa(int(AddrByteOrder.Uint16(address[4:]))))
}

func (address Addr) IsLocalhost() bool {
	if len(address) < 4 {
		return false
	}
	return address[0] == 127 && address[1] == 0 && address[2] == 0 && address[3] == 1
}

// length of address MUST NOT be less than 6
func (address Addr) WithStatus(status ServiceStatus) []byte {
	return []byte{address[0], address[1], address[2], address[3], address[4], address[5], byte(status)}
}

// type closederr struct {
// 	code   uint16
// 	reason string
// }

// func (err closederr) Error() string {
// 	return suckutils.Concat("closeframe statuscode: ", strconv.Itoa(int(err.code)), "; reason: ", err.reason)
// }
