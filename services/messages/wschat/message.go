package main

import (
	"io"
	"project/services/messages/messagestypes"
	"project/wsconnector"
	"time"

	"github.com/gobwas/ws"
)

type message struct {
	UserId  userid                           `json:"userid"`
	ChatId  string                           `json:"chatid"`
	Type    messagestypes.MessageContentType `json:"mtype"`
	ErrCode int                              `json:"type"`
	Data    []byte                           `json:"data"`
	Time    time.Time                        `json:"time"`
}

func (*wsconn) NewMessage() wsconnector.MessageReader {
	return &message{}
}

func (m *message) Read(r io.Reader, h ws.Header) error {
	return wsconnector.ReadAndDecodeJson(r, m)
}
