package main

import (
	"io"
	"project/test/messages/messagestypes"
	"project/test/wsconnector"
	"time"

	"github.com/gobwas/ws"
)

type message struct {
	UserId string                           `json:"userid"`
	ChatId string                           `json:"chatid"`
	Type   messagestypes.MessageContentType `json:"type"`
	Data   []byte                           `json:"data"`
	Time   time.Time                        `json:"time"`
}

func (*wsconn) NewMessage() wsconnector.MessageReader {
	return &message{}
}

func (m *message) Read(r io.Reader, h ws.Header) error {
	return wsconnector.ReadAndDecodeJson(r, m)
}
