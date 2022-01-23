package types

import (
	"errors"
	"net"
)

type Logger interface {
	Debug(string, string)
	Info(string, string)
	Warning(string, string)
	Error(string, error)
}

type NetProtocol byte

const (
	NetProtocolUnix         NetProtocol = 1
	NetProtocolTcp          NetProtocol = 2
	NetProtocolNil          NetProtocol = 3 // for non-listeners
	NetProtocolNonlocalUnix NetProtocol = 4
)

func (np NetProtocol) String() string {
	switch np {
	case NetProtocolTcp:
		return "tcp"
	case NetProtocolUnix:
		return "unix"
	case NetProtocolNil:
		return "nil"
	case NetProtocolNonlocalUnix:
		return "nonlocalunix"
	}
	return ""
}

func (np NetProtocol) Verify(addr string) bool {
	switch np {
	case NetProtocolTcp:
		if net.ParseIP(addr) == nil {
			return false
		}
		return true
	case NetProtocolUnix:
		if (addr)[:5] == "/tmp/" && (addr)[len(addr)-5:] == ".sock" {
			return true
		}
	case NetProtocolNonlocalUnix:
		if (addr)[:5] == "/tmp/" && (addr)[len(addr)-5:] == ".sock" {
			return true
		}
	case NetProtocolNil:
		if len(addr) == 0 {
			return true
		}

	}
	return false
}

type OperationCode byte

const (
	// []byte{opcode, statuscode}
	OperationCodeMyStatusChanged OperationCode = 1
	// []byte{opcode, len(pubname), pubname, len(pubname), pubname, ...}
	OperationCodeUnsubscribeFromServices OperationCode = 3
	// []byte{opcode, len(pubname), pubname, len(pubname), pubname, ...}
	OperationCodeSubscribeToServices OperationCode = 4
	// pubinfo := []byte{statuscode, len(addr), addr, pubname},
	// message := []byte{opcode, len(pubinfo), pubinfo, len(pubinfo), pubinfo, ...}
	OperationCodeUpdatePubs OperationCode = 6
	// []byte{opcode}
	OperationCodeGiveMeOuterAddr OperationCode = 8
	// []byte{opcode, len(addr), addr}
	OperationCodeSetOutsideAddr OperationCode = 9
	// []byte{opcode}
	OperationCodeImSupended OperationCode = 5
	// []byte{opcode}
	OperationCodePing OperationCode = 7
	// []byte{opcode}
	OperationCodeOK    OperationCode = 2
	OperationCodeNOTOK OperationCode = 10
)

// no total length and opcode in return slice
func FormatOpcodeUpdatePubMessage(servname []byte, address []byte, newstatus ServiceStatus) []byte {
	if len(servname) == 0 || len(address) == 0 {
		return nil
	}
	return append(append(append(make([]byte, 0, len(address)+len(servname)+2), byte(newstatus), byte(len(address))), address...), servname...)
}

// servname, address, newstatus, error
func UnformatOpcodeUpdatePubMessage(raw []byte) ([]byte, []byte, ServiceStatus, error) {
	if len(raw) < 4 {
		return nil, nil, 0, errors.New("unformatted data")
	}
	status := ServiceStatus(raw[0])
	if len(raw) < (int(raw[1]) + 3) { // 1 for pubname
		return nil, nil, 0, errors.New("unformatted data")
	}
	return raw[2+int(raw[1]):], raw[2 : 2+int(raw[1])], status, nil
}

// with no len specified
func FormatAddress(netw NetProtocol, addr string) []byte {
	addr_byte := []byte(addr)
	formatted := make([]byte, 1, 1+len(addr_byte))
	formatted[0] = byte(netw)
	return append(formatted, addr_byte...)
}

// with no len specified
func UnformatAddress(raw []byte) (NetProtocol, string, error) {
	if len(raw) == 0 {
		return 0, "", errors.New("zero raw addr length")
	}
	return NetProtocol(raw[0]), string(raw[1:]), nil
}

//errors.New(suckutils.Concat("cant verify \"", addr, "\" as ", NetProtocol(raw[0]).String(), "\" protocol")

type ServiceStatus byte

const (
	StatusOff       ServiceStatus = 0
	StatusSuspended ServiceStatus = 1
	StatusOn        ServiceStatus = 2
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

func SeparatePayload(payload []byte) [][]byte {
	if len(payload) == 0 {
		return nil
	}
	items := make([][]byte, 0, 4)
	for i := 0; i < len(payload); {
		length := int(payload[i])
		if i+length+1 > len(payload) {
			return nil
		}
		items = append(items, payload[i+1:i+length+1])
		i = length + 1 + i
	}
	return items
}

func ConcatPayload(pieces ...[]byte) []byte {
	if len(pieces) == 0 {
		return nil
	}
	length := 0
	for i := 0; i < len(pieces); i++ {
		length += len(pieces[i]) + 1
	}
	payload := make([]byte, 0, length)
	for _, piece := range pieces {
		if len(piece) == 0 {
			continue
		}
		payload = append(append(payload, byte(len(piece))), piece...)
	}
	return payload
}

// func (sn ServiceName) Local() string {
// 	return suckutils.ConcatTwo("local.", string(sn))
// }

// func (sn ServiceName) LocalSub() string {
// 	return suckutils.ConcatTwo("local.sub.", string(sn))
// }

// func (sn ServiceName) Remote() string {
// 	return suckutils.ConcatTwo("remote.", string(sn))
// }

// func (sn ServiceName) RemoteSub() string {
// 	return suckutils.ConcatTwo("remote.sub", string(sn))
// }

// type Addr []byte
// type IPv4withPort Addr
// type Port Addr
// type IPv4 Addr
// type Unix []byte // Unix[0]=len(unixAddress)

// var AddrByteOrder = binary.LittleEndian

// func ParseIPv4withPort(addr string) IPv4withPort {
// 	return parseipv4(addr, true)
// }

// func ParseIPv4(addr string) IPv4 {
// 	return parseipv4(addr, false)
// }

// func parseipv4(addr string, withport bool) []byte {
// 	if len(addr) == 0 {
// 		return nil
// 	}
// 	var address []byte
// 	pieces := strings.Split(addr, ":")
// 	if withport {
// 		if len(pieces) != 2 {
// 			return nil
// 		}
// 		address = make([]byte, 6)
// 		port, err := strconv.ParseUint(pieces[1], 10, 16)
// 		if err != nil {
// 			return nil
// 		}
// 		AddrByteOrder.PutUint16(address[4:], uint16(port))
// 	} else {
// 		address = make([]byte, 4)
// 	}
// 	ipv4 := net.ParseIP(pieces[0]).To4()
// 	if ipv4 == nil {
// 		return nil
// 	}
// 	copy(address[0:4], ipv4)
// 	return address
// }

// func CheckIPv4withPort(addr string) bool {
// 	if len(addr) < 8 {
// 		return false
// 	}
// 	pieces := strings.Split(addr, ":")
// 	if len(pieces) != 2 {
// 		return false
// 	}
// 	if _, err := strconv.ParseUint(pieces[1], 10, 16); err != nil {
// 		return false
// 	}
// 	if net.ParseIP(pieces[0]) == nil {
// 		return false
// 	}
// 	return true
// }

// func (address IPv4) String() string {
// 	return Addr(address).String()
// }
// func (address IPv4withPort) String() string {
// 	return Addr(address).String()
// }
// func (port Port) String() string {
// 	return Addr(port).String()
// }

// func (address Addr) String() string {
// 	switch len(address) {
// 	case 4:
// 		return net.IPv4(address[0], address[1], address[2], address[3]).String()
// 	case 2:
// 		return strconv.Itoa(int(AddrByteOrder.Uint16(address)))
// 	case 6:
// 		suckutils.ConcatThree(net.IPv4(address[0], address[1], address[2], address[3]).String(), ":", strconv.Itoa(int(AddrByteOrder.Uint16(address[4:]))))
// 	}
// 	return ""
// }

// func (address IPv4withPort) Port() Port {
// 	if len(address) < 6 {
// 		return nil
// 	}
// 	return Port(address[4:])
// }

// func (address Addr) IsLocalhost() bool {
// 	if len(address) < 4 {
// 		return false
// 	}
// 	return address[0] == 127 && address[1] == 0 && address[2] == 0 && address[3] == 1
// }

// func (port Port) NewHost(newhost IPv4) IPv4withPort {
// 	if len(port) != 2 || len(newhost) < 4 {
// 		return nil
// 	}
// 	addr := make([]byte, 6)
// 	copy(addr[0:4], IPv4withPort(newhost[0:4]))
// 	copy(addr[4:], port)
// 	return addr
// }

// func (address IPv4withPort) WithStatus(status ServiceStatus) []byte {
// 	if len(address) < 6 {
// 		return nil
// 	}
// 	return []byte{address[0], address[1], address[2], address[3], address[4], address[5], byte(status)}
// }

// func (address IPv4withPort) NewHost(newhost IPv4) IPv4withPort {
// 	if len(address) < 4 || len(newhost) < 4 {
// 		return nil
// 	}
// 	copy(address[0:4], IPv4withPort(newhost[0:4]))
// 	return address
// }

// func (address IPv4withPort) GetHost() IPv4 {
// 	if len(address) < 4 {
// 		return nil
// 	}
// 	return IPv4(address[0:4])
// }
// func ParseUnix(unixaddr string) Unix {
// 	if len(unixaddr) == 0 || len(unixaddr) > 255 {
// 		return nil
// 	}
// 	b := []byte(unixaddr)
// 	unix := make([]byte, 0, len(b)+1)
// 	unix[0] = uint8(len(b))
// 	return append(unix, b...)
// }

// func GenerateMemcStatusValue(ip IPv4withPort, unix Unix, status ServiceStatus) []byte {
// 	if len(ip) != 6 || len(unix) == 0 {
// 		return nil
// 	}
// 	v := make([]byte, 0, 7+len(unix))
// 	return append(append(append(v, ip...), unix...), byte(status))
// }

// type closederr struct {
// 	code   uint16
// 	reason string
// }

// func (err closederr) Error() string {
// 	return suckutils.Concat("closeframe statuscode: ", strconv.Itoa(int(err.code)), "; reason: ", err.reason)
// }
