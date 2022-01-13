package connector

import (
	"encoding/binary"
	"errors"
	"time"
)

const maxlength = 4096

type BasicMessage struct {
	Payload []byte
}

func (msg BasicMessage) Read(conn ConnReader) error {

	buf := make([]byte, 4)
	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, err := conn.Read(buf)
	if err != nil {
		return err
	}
	msglength := binary.BigEndian.Uint32(buf)
	if msglength > maxlength {
		return errors.New("payload too long")
	}
	msg.Payload = make([]byte, msglength)
	//conn.SetReadDeadline(time.Now().Add((time.Millisecond * 700) * (time.Duration((msglength / 1024) + 1))))
	conn.SetReadDeadline(time.Now().Add(time.Second * 2))
	_, err = conn.Read(buf)
	return err
}

// payload not allocated
func NewBasicMessage() BasicMessage {
	return BasicMessage{}
}

func FormatBasicMessage(message []byte) []byte {
	formattedmsg := make([]byte, 4, 4+len(message))
	if len(message) > 0 {
		binary.BigEndian.PutUint32(formattedmsg, uint32(len(message)))
		return append(formattedmsg, message...)
	}
	return formattedmsg
}
