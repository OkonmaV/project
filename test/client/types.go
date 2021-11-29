package client

import (
	"encoding/binary"
	"net"
	"strconv"
	"strings"

	"github.com/big-larry/suckutils"
)

type OperationCode byte

const (
	OperationCodeSetMyStatusOff       OperationCode = 1
	OperationCodeSetMyStatusSuspended OperationCode = 2
	OperationCodeSetMyStatusOn        OperationCode = 3
	OperationCodeSubscribeToServices  OperationCode = 4
	OperationCodeSetPubAddresses      OperationCode = 5
	OperationCodeUpdatePubStatus      OperationCode = 6 // opcode + one byte for new pub's status + subscription servicename + subscription service addr
	OperationCodeError                OperationCode = 7 // must not be handled but printed at service-caller, for debugging errors in caller's code
	OperationCodeGiveMeOotsideAddr    OperationCode = 8
)

type ServiceStatus byte

const (
	StatusOff       ServiceStatus = 0
	StatusSuspended ServiceStatus = 1
	StatusOn        ServiceStatus = 2
)

type Network byte

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

type ServiceName string // only raw - without "local." or "remote."

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
type IPv4withPort Addr
type Port Addr
type IPv4 Addr
type Unix []byte // Unix[0]=len(unixAddress)

var AddrByteOrder = binary.LittleEndian

func ParseIPv4withPort(addr string) IPv4withPort {
	return parseipv4(addr, true)
}

func ParseIPv4(addr string) IPv4 {
	return parseipv4(addr, false)
}

func parseipv4(addr string, withport bool) []byte {
	if len(addr) == 0 {
		return nil
	}
	var address []byte
	pieces := strings.Split(addr, ":")
	if withport {
		if len(pieces) != 2 {
			return nil
		}
		address = make([]byte, 6)
		port, err := strconv.ParseUint(pieces[1], 10, 16)
		if err != nil {
			return nil
		}
		AddrByteOrder.PutUint16(address[4:], uint16(port))
	} else {
		address = make([]byte, 4)
	}
	ipv4 := net.ParseIP(pieces[0]).To4()
	if ipv4 == nil {
		return nil
	}
	copy(address[0:4], ipv4)
	return address
}

func CheckIPv4withPort(addr string) bool {
	if len(addr) < 8 {
		return false
	}
	pieces := strings.Split(addr, ":")
	if len(pieces) != 2 {
		return false
	}
	if _, err := strconv.ParseUint(pieces[1], 10, 16); err != nil {
		return false
	}
	if net.ParseIP(pieces[0]) == nil {
		return false
	}
	return true
}

func (address IPv4) String() string {
	return Addr(address).String()
}
func (address IPv4withPort) String() string {
	return Addr(address).String()
}
func (port Port) String() string {
	return Addr(port).String()
}

func (address Addr) String() string {
	switch len(address) {
	case 4:
		return net.IPv4(address[0], address[1], address[2], address[3]).String()
	case 2:
		return strconv.Itoa(int(AddrByteOrder.Uint16(address)))
	case 6:
		suckutils.ConcatThree(net.IPv4(address[0], address[1], address[2], address[3]).String(), ":", strconv.Itoa(int(AddrByteOrder.Uint16(address[4:]))))
	}
	return ""
}

func (address IPv4withPort) Port() Port {
	if len(address) < 6 {
		return nil
	}
	return Port(address[4:])
}

func (address Addr) IsLocalhost() bool {
	if len(address) < 4 {
		return false
	}
	return address[0] == 127 && address[1] == 0 && address[2] == 0 && address[3] == 1
}

func (port Port) NewHost(newhost IPv4) IPv4withPort {
	if len(port) != 2 || len(newhost) < 4 {
		return nil
	}
	addr := make([]byte, 6)
	copy(addr[0:4], IPv4withPort(newhost[0:4]))
	copy(addr[4:], port)
	return addr
}

func (address IPv4withPort) WithStatus(status ServiceStatus) []byte {
	if len(address) < 6 {
		return nil
	}
	return []byte{address[0], address[1], address[2], address[3], address[4], address[5], byte(status)}
}

func (address IPv4withPort) NewHost(newhost IPv4) IPv4withPort {
	if len(address) < 4 || len(newhost) < 4 {
		return nil
	}
	copy(address[0:4], IPv4withPort(newhost[0:4]))
	return address
}

func (address IPv4withPort) GetHost() IPv4 {
	if len(address) < 4 {
		return nil
	}
	return IPv4(address[0:4])
}
func ParseUnix(unixaddr string) Unix {
	if len(unixaddr) == 0 || len(unixaddr) > 255 {
		return nil
	}
	b := []byte(unixaddr)
	unix := make([]byte, 0, len(b)+1)
	unix[0] = uint8(len(b))
	return append(unix, b...)
}

func GenerateMemcStatusValue(ip IPv4withPort, unix Unix, status ServiceStatus) []byte {
	if len(ip) != 6 || len(unix) == 0 {
		return nil
	}
	v := make([]byte, 0, 7+len(unix))
	return append(append(append(v, ip...), unix...), byte(status))
}

type closederr struct {
	code   uint16
	reason string
}

func (err closederr) Error() string {
	return suckutils.Concat("closeframe statuscode: ", strconv.Itoa(int(err.code)), "; reason: ", err.reason)
}
