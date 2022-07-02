package protocol

import (
	"encoding/binary"
)

type MessageType byte
type AppID uint16
type ConnUID uint32

var byteOrder = binary.BigEndian

const (
	TypeInstall       MessageType = 1
	TypeConnect       MessageType = 2
	TypeText          MessageType = 3
	TypeError         MessageType = 4
	TypeDisconnection MessageType = 5
	TypeRegistration  MessageType = 6
	TypeTimestamp     MessageType = 7
	TypeCreate        MessageType = 8
	TypeUpdate        MessageType = 9
	TypeSettingsReq   MessageType = 10
)

func (mt MessageType) String() string {
	switch mt {
	case TypeInstall:
		return "Install"
	case TypeConnect:
		return "Connect"
	case TypeText:
		return "Text"
	case TypeError:
		return "Error"
	case TypeDisconnection:
		return "Disconnection"
	case TypeRegistration:
		return "Registration"
	case TypeTimestamp:
		return "Timestamp"
	case TypeCreate:
		return "Create"
	case TypeUpdate:
		return "Update"
	case TypeSettingsReq:
		return "SettingsReq"
	}
	return "Unknown"
}

// Протокол:
// client <--> appserver : 1 byte msgtype, 1 byte reserved, 2 bytes appID, 8 bytes timestamp, 2 bytes headers len, 4 bytes body len, дальше хедеры и тело
// appserver <--> app : 1 byte msgtype, 1 byte reserved, 4 bytes connUID, 8 bytes timestamp, 2 bytes headers len, 4 bytes body len, дальше хедеры и тело
