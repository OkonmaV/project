package main

import (
	"errors"
	"fmt"

	"project/logs/logger"
	"project/wsservice"

	"project/wsconnector"

	"time"

	"github.com/big-larry/suckutils"
)

type wsconn struct {
	userId string
	conn   wsservice.WSconn

	srvc *service
	l    logger.Logger
}

// wsservice.Handler interface implementation
func (wsc *wsconn) HandleNewConnection(conn wsservice.WSconn) error {
	wsc.l.Debug("New conn", suckutils.ConcatTwo("Connected from ", conn.RemoteAddr().String()))
	wsc.srvc.addconn(wsc)
	wsc.conn = conn
	return nil
}

// wsservice.Handler {wsconnector.WsHandler} interface implementation
func (wsc *wsconn) Handle(msg interface{}) error {
	wsc.l.Info("NEW MESSAGE", fmt.Sprint(msg))

	m := msg.(message)

	var err error
	var errmsg []byte
	mOp := m["messagetype"].(uint8)
	switch mOp {
	case 1: // GET
		msgs, err := wsconnector.EncodeJson(wsc.srvc.getmessages())
		if err != nil {
			wsc.l.Error("Marshalling all messages", err)
			m["ERROR"] = err.Error()
			break
		}
		wsc.conn.Send(msgs)
		return nil
	case 2: // SEND
		m["time"] = time.Now()
		m["userid"] = wsc.userId
		wsc.srvc.addmessage(m)

		confirmed_msg, err := wsconnector.EncodeJson(m)
		if err != nil {
			wsc.l.Error("Marshalling message", errors.New(fmt.Sprint("message:", m, "err:", err.Error())))
			m["ERROR"] = err.Error()
		}

		wsc.srvc.sendToAll(confirmed_msg)
		return nil

	default:
		wsc.l.Error("Message", errors.New(fmt.Sprint("Unknown opcode:", m["opcode"])))
		m["ERROR"] = fmt.Sprint("UNKNOWN OPCODE (field named opcode)", m["opcode"], ", I KNOW OPs: OPGET=1 and OPSEND=2 (uint8 typed)")
	}

	errmsg, err = wsconnector.EncodeJson(m)
	if err != nil {
		panic(err)
	}
	return wsc.conn.Send(errmsg)
}

func (srvc *service) sendToAll(msg []byte) {
	srvc.RLock()
	defer srvc.RUnlock()

	for _, wsc := range srvc.users {
		if err := wsc.conn.Send(msg); err != nil {
			wsc.l.Error("Send", err)
		}
	}
}

// wsservice.Handler {wsconnector.WsHandler} interface implementation
func (wsc *wsconn) HandleClose(err error) {
	wsc.l.Error("oh shit", err)
	wsc.srvc.deleteconn(wsc)
}
