package protocol

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"

	"github.com/gobwas/ws"
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

type ClientMessage struct {
	Type MessageType // 1 byte
	// reserved1 byte        // 1 byte
	ApplicationID AppID
	Headers       []byte // uint16 len
	Body          []byte // uint32 len
	Timestamp     int64  // так как в ответах от сервера всегда будет, то пихать его в хедеры странно
}

const Client_message_head_len = 18

func (m *ClientMessage) Read(conn net.Conn) error {

	head := make([]byte, Client_message_head_len)
	conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	n, err := conn.Read(head)
	if err != nil {
		return err
	}
	if n != Client_message_head_len {
		return errors.New("readed less head bytes than expected")
	}

	headers_len := byteOrder.Uint16(head[12:14])
	body_len := byteOrder.Uint32(head[14:18])

	m.Headers = make([]byte, headers_len)
	conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	n, err = conn.Read(m.Headers)
	if err != nil {
		return err
	}
	if n != int(headers_len) {
		return errors.New("readed less headers bytes than expected")
	}

	m.Body = make([]byte, body_len)
	conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	n, err = conn.Read(m.Body)
	if err != nil {
		return err
	}
	if n != int(body_len) {
		return errors.New("readed less body bytes than expected")
	}

	m.Type = MessageType(head[0])
	m.ApplicationID = AppID(byteOrder.Uint16(head[2:4]))
	m.Timestamp = int64(byteOrder.Uint64(head[4:12])) // нужно ли читать таймстемп присланный клиентом???

	return nil
}

func (m *ClientMessage) Encode() ([]byte, error) {
	return EncodeClientMessage(m.Type, m.ApplicationID, m.Timestamp, m.Headers, m.Body)
}

func EncodeClientMessage(messagetype MessageType, appID AppID, timestamp int64, headers []byte, body []byte) ([]byte, error) {
	if len(headers) > 65535 {
		return nil, errors.New("headers overflows (len specified by uint16)")
	}
	if len(body) > 4294967295 {
		return nil, errors.New("body overflows (len specified by uint32)")
	}

	encoded := make([]byte, Client_message_head_len+len(headers)+len(body))

	encoded[0] = byte(messagetype)

	byteOrder.PutUint16(encoded[2:4], uint16(appID))
	byteOrder.PutUint64(encoded[4:12], uint64(timestamp)) // нужно ли читать таймстемп присланный клиентом???
	byteOrder.PutUint16(encoded[12:14], uint16(len(headers)))
	byteOrder.PutUint32(encoded[14:18], uint32(len(body)))

	copy(encoded[Client_message_head_len:Client_message_head_len+len(headers)], headers)
	copy(encoded[Client_message_head_len+len(headers):], body)

	return encoded, nil
}

func DecodeClientMessage(rawmessage []byte) (*ClientMessage, error) {
	if len(rawmessage) < Client_message_head_len {
		return nil, errors.New("weird data(raw message does not satisfy min head len)")
	}
	headers_len := byteOrder.Uint16(rawmessage[12:14])
	body_len := byteOrder.Uint32(rawmessage[14:18])
	if len(rawmessage) != Client_message_head_len+int(headers_len)+int(body_len) {
		return nil, errors.New("weird data(raw message is shorter/longer than specified head, body and headers lengths)")
	}

	m := &ClientMessage{
		Type:          MessageType(rawmessage[0]),
		ApplicationID: AppID(byteOrder.Uint16(rawmessage[2:4])),
		Timestamp:     int64(byteOrder.Uint64(rawmessage[4:12])),
		Headers:       rawmessage[Client_message_head_len : Client_message_head_len+headers_len],
		Body:          rawmessage[Client_message_head_len+headers_len:],
	}

	//m.Headers = make([]byte, headers_len)
	//copy(m.Headers, rawmessage[Client_message_head_len:Client_message_head_len+headers_len])

	//m.Body = make([]byte, body_len)
	//copy(m.Body, rawmessage[Client_message_head_len+headers_len:])
	return m, nil
}

type AppMessage struct {
	Type MessageType // 1 byte
	// reserved1 // 1 byte
	ConnectionUID ConnUID
	Headers       []byte // uint16 len
	Body          []byte // uint32 len
	Timestamp     int64
}

const App_message_head_len = 20
const Max_ConnUID = 16777215

func (m *AppMessage) Read(conn net.Conn) error {

	head := make([]byte, App_message_head_len)
	conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	n, err := conn.Read(head)
	if err != nil {
		return err
	}
	if n != App_message_head_len {
		return errors.New("readed less head bytes than expected")
	}

	headers_len := byteOrder.Uint16(head[14:16])
	body_len := byteOrder.Uint32(head[16:20])

	m.Headers = make([]byte, headers_len)
	conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	n, err = conn.Read(m.Headers)
	if err != nil {
		return err
	}
	if n != int(headers_len) {
		return errors.New("readed less headers bytes than expected")
	}

	m.Body = make([]byte, body_len)
	conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	n, err = conn.Read(m.Body)
	if err != nil {
		return err
	}
	if n != int(body_len) {
		return errors.New("readed less body bytes than expected")
	}

	m.Type = MessageType(head[0])
	m.ConnectionUID = ConnUID(byteOrder.Uint32(head[2:6]))
	m.Timestamp = int64(byteOrder.Uint64(head[6:14]))

	return nil
}

func (m *AppMessage) Encode() ([]byte, error) {
	return EncodeAppMessage(m.Type, m.ConnectionUID, m.Timestamp, m.Headers, m.Body)
}

func EncodeAppMessage(messagetype MessageType, connUID ConnUID, timestamp int64, headers []byte, body []byte) ([]byte, error) {
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

	encoded := make([]byte, App_message_head_len+len(headers)+len(body))

	encoded[0] = byte(messagetype)

	byteOrder.PutUint32(encoded[2:6], uint32(connUID))
	byteOrder.PutUint64(encoded[6:14], uint64(timestamp))
	byteOrder.PutUint16(encoded[14:16], uint16(len(headers)))
	byteOrder.PutUint32(encoded[16:20], uint32(len(body)))

	copy(encoded[App_message_head_len:App_message_head_len+len(headers)], headers)
	copy(encoded[App_message_head_len+len(headers):], body)

	return encoded, nil
}

func DecodeAppMessage(rawmessage []byte) (*AppMessage, error) {
	if len(rawmessage) < App_message_head_len {
		return nil, errors.New("weird data(raw message does not satisfy min head len)")
	}
	headers_len := byteOrder.Uint16(rawmessage[14:16])
	body_len := byteOrder.Uint32(rawmessage[16:20])
	if len(rawmessage) != App_message_head_len+int(headers_len)+int(body_len) {
		return nil, errors.New("weird data(raw message is shorter/longer than specified head, body and headers lengths)")
	}

	return &AppMessage{
		Type:          MessageType(rawmessage[0]),
		ConnectionUID: ConnUID(byteOrder.Uint32(rawmessage[2:6])),
		Headers:       rawmessage[App_message_head_len : App_message_head_len+headers_len],
		Body:          rawmessage[App_message_head_len+headers_len:],
		Timestamp:     int64(byteOrder.Uint64(rawmessage[6:14])),
	}, nil
}

type AppServerMessage struct {
	Type MessageType // 1 byte
	// reserved1   byte          // 1 byte
	ApplicationID  AppID
	ConnectionUID  ConnUID
	Generation     byte
	RawMessageData []byte // whole headers and body in unparsed, virgin form, with their lengths specified
	Timestamp      int64
}

// ONLY FOR READING CLIENT MESSAGE!!!!!! wsconnector.MessageReader implementation
func (m *AppServerMessage) Read(r io.Reader, h ws.Header) error {
	if h.Length < Client_message_head_len {
		return errors.New("payload is less than client message head len")
	}
	payload := make([]byte, h.Length)
	_, err := io.ReadFull(r, payload)
	if err != nil {
		return err
	}

	headers_len := byteOrder.Uint16(payload[12:14])
	body_len := byteOrder.Uint32(payload[14:18])
	if h.Length != int64(Client_message_head_len)+int64(headers_len)+int64(body_len) {
		return errors.New("wsheader.length does not match with specified len in protocol's message head")
	}

	m.Type = MessageType(payload[0])
	m.ApplicationID = AppID(byteOrder.Uint16(payload[2:4]))
	m.Timestamp = int64(byteOrder.Uint64(payload[4:12])) // нужно ли читать таймстемп присланный клиентом???
	m.RawMessageData = payload[12:]

	return nil
}

func DecodeClientMessageToAppServerMessage(rawmessage []byte) (*AppServerMessage, error) {
	if len(rawmessage) < Client_message_head_len {
		return nil, errors.New("weird data(raw message does not satisfy min head len)")
	}
	headers_len := byteOrder.Uint16(rawmessage[12:14])
	body_len := byteOrder.Uint32(rawmessage[14:18])
	if len(rawmessage) != Client_message_head_len+int(headers_len)+int(body_len) {
		println(Client_message_head_len, headers_len, body_len)
		return nil, errors.New("weird data(raw message is shorter/longer than specified head, body and headers lengths)")
	}

	return &AppServerMessage{
		Type:           MessageType(rawmessage[0]),
		ApplicationID:  AppID(byteOrder.Uint16(rawmessage[2:4])),
		Timestamp:      int64(byteOrder.Uint64(rawmessage[4:12])), // нужно ли читать таймстемп присланный клиентом???
		RawMessageData: rawmessage[12:],
	}, nil
}

func DecodeAppMessageToAppServerMessage(rawmessage []byte) (*AppServerMessage, error) {
	if len(rawmessage) < App_message_head_len {
		return nil, errors.New("weird data(raw message does not satisfy min head len)")
	}
	headers_len := byteOrder.Uint16(rawmessage[14:16])
	body_len := byteOrder.Uint32(rawmessage[16:20])
	if len(rawmessage) != App_message_head_len+int(headers_len)+int(body_len) {
		return nil, errors.New("weird data(raw message is shorter/longer than specified head, body and headers lengths)")
	}

	return &AppServerMessage{
		Type:           MessageType(rawmessage[0]),
		Generation:     rawmessage[2],
		ConnectionUID:  ConnUID(byteOrder.Uint32(rawmessage[2:6]) >> 8),
		Timestamp:      int64(byteOrder.Uint64(rawmessage[6:14])),
		RawMessageData: rawmessage[6:],
	}, nil
}

func (m *AppServerMessage) EncodeToClientMessage() []byte {
	encoded := make([]byte, Client_message_head_len-6+len(m.RawMessageData))

	encoded[0] = byte(m.Type)

	byteOrder.PutUint16(encoded[2:4], uint16(m.ApplicationID))
	byteOrder.PutUint64(encoded[4:12], uint64(m.Timestamp))

	copy(encoded[12:], m.RawMessageData)

	return encoded
}

func (m *AppServerMessage) EncodeToAppMessage() ([]byte, error) {
	if m.ConnectionUID > Max_ConnUID {
		return nil, errors.New("connuid must be less then 16777215")
	}
	encoded := make([]byte, App_message_head_len-6+len(m.RawMessageData))

	encoded[0] = byte(m.Type)

	byteOrder.PutUint32(encoded[2:6], uint32(m.ConnectionUID))
	byteOrder.PutUint64(encoded[6:14], uint64(m.Timestamp))
	copy(encoded[14:], m.RawMessageData)

	return encoded, nil
}
