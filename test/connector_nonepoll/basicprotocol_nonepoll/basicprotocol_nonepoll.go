package basicprotocol_nonepoll

import (
	"encoding/binary"
	"errors"
	connectornonepoll "project/test/connector_nonepoll"
)

type BasicMessage struct {
	payload []byte
}

func (msg *BasicMessage) Read(conn connectornonepoll.ConnectorConnReader) error {

	buf := make([]byte, 4)
	//conn.SetReadDeadline(time.Now().Add(time.Second))
	_, err := conn.Read(buf)
	if err != nil {
		return err
	}
	msglength := binary.BigEndian.Uint32(buf)
	if msglength > 4096 { // TODO
		return errors.New("payload too long")
	}
	msg.payload = make([]byte, msglength)
	//conn.SetReadDeadline(time.Now().Add((time.Millisecond * 700) * (time.Duration((msglength / 1024) + 1))))
	//conn.SetReadDeadline(time.Now().Add(time.Second * 2))
	_, err = conn.Read(buf)
	return err
}

// payload not allocated
func NewMessage() *BasicMessage {
	return &BasicMessage{}
}

func FormatMessage(message []byte) []byte {
	formattedmsg := make([]byte, 4, 4+len(message))
	if len(message) > 0 {
		binary.BigEndian.PutUint32(formattedmsg, uint32(len(message)))
		return append(formattedmsg, message...)
	}
	return formattedmsg
}
