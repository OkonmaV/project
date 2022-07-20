package protocol

import (
	"encoding/binary"
)

type MessageType byte
type ErrorCode byte
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

	TypeOK MessageType = 15 // я хуй знает как назвать

	TypeRedirection  MessageType = 11
	TypeToken        MessageType = 12
	TypeGrant        MessageType = 16
	TypeAuthData     MessageType = 13
	TypeIntroduction MessageType = 14
)

const (
	ErrCodeNil                 ErrorCode = 0
	ErrCodeNotFound            ErrorCode = 1
	ErrCodeBadRequest          ErrorCode = 2
	ErrCodeInternalServerError ErrorCode = 3
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
	case TypeRedirection:
		return "Redirection"
	case TypeToken:
		return "Token"
	case TypeAuthData:
		return "AuthData"
	case TypeIntroduction:
		return "Introduction"
	case TypeOK:
		return "OK"
	case TypeGrant:
		return "Grant"
	}
	return "Unknown"
}

func (ec ErrorCode) String() string {
	switch ec {
	case ErrCodeNotFound:
		return "NotFound"
	case ErrCodeBadRequest:
		return "BadRequest"
	case ErrCodeNil:
		return "Nil"
	case ErrCodeInternalServerError:
		return "InternalServerError"
	}
	return "Unknown"
}

type IdentityServerMessage_Headers struct {
	Grant string `json:"grant,omitempty"`

	App_Id     string `json:"appid,omitempty"`
	App_Secret string `json:"appsecret,omitempty"`

	Access_Token  string `json:"a_token,omitempty"`
	Refresh_Token string `json:"r_token,omitempty"`

	AuthCode string `json:"code,omitempty"`
}

// Протокол:
// client <--> appserver : 1 byte msgtype, 1 byte reserved, 2 bytes appID, 8 bytes timestamp, 2 bytes headers len, 4 bytes body len, дальше хедеры и тело
// appserver <--> app : 1 byte msgtype, 1 byte reserved, 4 bytes connUID, 8 bytes timestamp, 2 bytes headers len, 4 bytes body len, дальше хедеры и тело
