package main

import (
	"io"
	"project/wsconnector"

	"github.com/gobwas/ws"
)

// type message struct {
// 	Message map[string]interface{}
// }

// func (*wsconn) NewMessage() wsconnector.MessageReader {
// 	return &message{Message: make(map[string]interface{})}
// }

// func (m *message) Read(r io.Reader, h ws.Header) error {
// 	return wsconnector.ReadAndDecodeJson(r, &m.Message)
// }

type message map[string]interface{}

func (*wsconn) NewMessage() wsconnector.MessageReader {
	return make(message)
}

func (m message) Read(r io.Reader, h ws.Header) error {
	return wsconnector.ReadAndDecodeJson(r, &m)
}
