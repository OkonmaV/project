package protocol

import (
	"encoding/binary"
	"errors"
)

type MessageType byte
type AppID uint16
type ConnUID uint32

const (
	TypeInstall       MessageType = 1
	TypeConnect       MessageType = 2
	TypeText          MessageType = 3
	TypeError         MessageType = 4
	TypeDisconnection MessageType = 5
)

type ClientMessage struct {
	Type MessageType // 1 byte
	// reserved1 byte        // 1 byte
	ApplicationID AppID
	Headers       []byte // uint16 len
	Body          []byte // uint32 len
}

var byteOrder = binary.BigEndian

const client_message_head_len = 10 // TODO: delete this

func (m *ClientMessage) Encode() ([]byte, error) {
	return EncodeClientMessage(m.Type, m.ApplicationID, m.Headers, m.Body)
}

func EncodeClientMessage(messagetype MessageType, appID AppID, headers []byte, body []byte) ([]byte, error) {
	if len(headers) > 65535 {
		return nil, errors.New("headers overflows (len specified by uint16)")
	}
	if len(body) > 4294967295 {
		return nil, errors.New("body overflows (len specified by uint32)")
	}

	encoded := make([]byte, client_message_head_len+len(headers)+len(body))

	encoded[0] = byte(messagetype)

	byteOrder.PutUint16(encoded[2:4], uint16(appID))
	byteOrder.PutUint16(encoded[4:6], uint16(len(headers)))
	byteOrder.PutUint32(encoded[6:10], uint32(len(body)))

	copy(encoded[client_message_head_len:client_message_head_len+len(headers)], headers)
	copy(encoded[client_message_head_len+len(headers):], body)

	return encoded, nil
}

func DecodeClientMessage(rawmessage []byte) (*ClientMessage, error) {
	if len(rawmessage) < client_message_head_len {
		return nil, errors.New("weird data(raw message does not satisfy min head len)")
	}
	headers_len := byteOrder.Uint16(rawmessage[4:6])
	body_len := byteOrder.Uint32(rawmessage[6:10])
	if len(rawmessage) != client_message_head_len+int(headers_len)+int(body_len) {
		return nil, errors.New("weird data(raw message is shorter/longer than specified head, body and headers lengths)")
	}

	m := &ClientMessage{
		Type:          MessageType(rawmessage[0]),
		ApplicationID: AppID(byteOrder.Uint16(rawmessage[2:4])),
		Headers:       rawmessage[client_message_head_len : client_message_head_len+headers_len],
		Body:          rawmessage[client_message_head_len+headers_len:],
	}

	//m.Headers = make([]byte, headers_len)
	//copy(m.Headers, rawmessage[client_message_head_len:client_message_head_len+headers_len])

	//m.Body = make([]byte, body_len)
	//copy(m.Body, rawmessage[client_message_head_len+headers_len:])
	return m, nil
}

type AppMessage struct {
	Type MessageType // 1 byte
	// reserved1 // 1 byte
	ConnectionUID ConnUID
	Headers       []byte // uint16 len
	Body          []byte // uint32 len
}

const app_message_head_len = 12 // TODO: delete this
const Max_ConnUID = 16777215

func (m *AppMessage) Encode() ([]byte, error) {
	return EncodeAppMessage(m.Type, m.ConnectionUID, m.Headers, m.Body)
}

func EncodeAppMessage(messagetype MessageType, connUID ConnUID, headers []byte, body []byte) ([]byte, error) {
	// if connUID == 0 {
	// 	return nil, errors.New("connUID is zero")
	// }
	if connUID > Max_ConnUID {
		return nil, errors.New("connuid mus be less then 16777215")
	}
	if len(headers) > 65535 {
		return nil, errors.New("headers overflows (len specified by uint16)")
	}
	if len(body) > 4294967295 {
		return nil, errors.New("body overflows (len specified by uint32)")
	}

	encoded := make([]byte, app_message_head_len+len(headers)+len(body))

	encoded[0] = byte(messagetype)

	byteOrder.PutUint32(encoded[2:6], uint32(connUID))
	byteOrder.PutUint16(encoded[6:8], uint16(len(headers)))
	byteOrder.PutUint32(encoded[8:12], uint32(len(body)))

	copy(encoded[app_message_head_len:app_message_head_len+len(headers)], headers)
	copy(encoded[app_message_head_len+len(headers):], body)

	return encoded, nil
}

func DecodeAppMessage(rawmessage []byte) (*AppMessage, error) {
	if len(rawmessage) < app_message_head_len {
		return nil, errors.New("weird data(raw message does not satisfy min head len)")
	}
	headers_len := byteOrder.Uint16(rawmessage[6:8])
	body_len := byteOrder.Uint32(rawmessage[8:12])
	if len(rawmessage) != app_message_head_len+int(headers_len)+int(body_len) {
		return nil, errors.New("weird data(raw message is shorter/longer than specified head, body and headers lengths)")
	}

	m := &AppMessage{
		Type:          MessageType(rawmessage[0]),
		ConnectionUID: ConnUID(byteOrder.Uint32(rawmessage[2:6])),
		Headers:       rawmessage[app_message_head_len : app_message_head_len+headers_len],
		Body:          rawmessage[app_message_head_len+headers_len:],
	}

	return m, nil
}

type AppServerMessage struct {
	Type MessageType // 1 byte
	// reserved1   byte          // 1 byte
	ApplicationID  AppID
	ConnectionUID  ConnUID
	Generation     byte
	RawMessageData []byte // whole headers and body in unparsed, virgin form, with their lengths specified
}

func DecodeClientMessageToAppServerMessage(rawmessage []byte) (*AppServerMessage, error) {
	if len(rawmessage) < client_message_head_len {
		return nil, errors.New("weird data(raw message does not satisfy min head len)")
	}
	headers_len := byteOrder.Uint16(rawmessage[4:6])
	body_len := byteOrder.Uint32(rawmessage[6:10])
	if len(rawmessage) != client_message_head_len+int(headers_len)+int(body_len) {
		println(client_message_head_len, headers_len, body_len)
		return nil, errors.New("weird data(raw message is shorter/longer than specified head, body and headers lengths)")
	}

	return &AppServerMessage{
		Type:           MessageType(rawmessage[0]),
		ApplicationID:  AppID(byteOrder.Uint16(rawmessage[2:4])),
		RawMessageData: rawmessage[4:],
	}, nil
}

func DecodeAppMessageToAppServerMessage(rawmessage []byte) (*AppServerMessage, error) {
	if len(rawmessage) < app_message_head_len {
		return nil, errors.New("weird data(raw message does not satisfy min head len)")
	}
	headers_len := byteOrder.Uint16(rawmessage[6:8])
	body_len := byteOrder.Uint32(rawmessage[8:12])
	if len(rawmessage) != app_message_head_len+int(headers_len)+int(body_len) {
		return nil, errors.New("weird data(raw message is shorter/longer than specified head, body and headers lengths)")
	}

	return &AppServerMessage{
		Type:           MessageType(rawmessage[0]),
		ConnectionUID:  ConnUID(byteOrder.Uint32(rawmessage[2:6]) >> 8),
		Generation:     rawmessage[2],
		RawMessageData: rawmessage[6:],
	}, nil
}

func (m *AppServerMessage) EncodeToClientMessage() []byte {
	encoded := make([]byte, client_message_head_len-6+len(m.RawMessageData))

	encoded[0] = byte(m.Type)

	byteOrder.PutUint16(encoded[2:4], uint16(m.ApplicationID))

	copy(encoded[4:], m.RawMessageData)

	return encoded
}

func (m *AppServerMessage) EncodeToAppMessage() ([]byte, error) {
	if m.ConnectionUID > Max_ConnUID {
		return nil, errors.New("connuid must be less then 16777215")
	}
	encoded := make([]byte, app_message_head_len-6+len(m.RawMessageData))

	encoded[0] = byte(m.Type)

	byteOrder.PutUint32(encoded[2:6], uint32(m.ConnectionUID))
	copy(encoded[6:], m.RawMessageData)

	return encoded, nil
}
