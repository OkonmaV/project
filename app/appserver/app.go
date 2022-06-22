package main

import (
	"errors"
	"net"
	"project/app/protocol"
	"project/connector"
	"project/logs/logger"
	"sync"
	"time"

	"github.com/big-larry/suckutils"
)

type app struct {
	conns       []*connector.EpollReConnector
	appid       protocol.AppID
	servicename ServiceName
	clients     *clientsConnsList

	l logger.Logger
	sync.RWMutex
}

// sends only to the first successful sending, ignores other conns
func (a *app) SendToOne(message []byte) error {
	a.RLock()
	defer a.RUnlock()
	for _, conn := range a.conns {
		if err := conn.Send(message); err != nil {
			a.l.Error("Send", err)
			continue
		}
		return nil
	}
	return errNoAliveConns
}

func (a *app) SendToAll(message []byte) {
	a.RLock()
	defer a.RUnlock()
	for _, conn := range a.conns {
		if err := conn.Send(message); err != nil {
			a.l.Error("Send", err)
			continue
		}

	}
}

// app.mutex inside
func (a *app) connect(netw, addr string) {
	var recon *connector.EpollReConnector

	conn, err := net.DialTimeout(netw, addr, time.Second)
	if err != nil {
		a.l.Error("Dial", errors.New(suckutils.ConcatTwo("err catched, reconnect inited, err: ", err.Error())))
		goto conn_failed
	}
	if recon, err = connector.NewEpollReConnector(conn, a, nil, a.doAfterReconnect); err != nil {
		a.l.Error("NewEpollReConnector", errors.New(suckutils.ConcatTwo("err catched, reconnect inited, err: ", err.Error())))
		goto conn_failed
	}
	if err = recon.StartServing(); err != nil {
		recon.ClearFromCache()
		a.l.Error("StartServing", errors.New(suckutils.ConcatTwo("err catched, reconnect inited, err: ", err.Error())))
		goto conn_failed
	}
	goto conn_succeeded

conn_failed:
	recon = connector.DialWithReconnect(netw, addr, a, nil, a.doAfterReconnect)

conn_succeeded:
	a.l.Info("Conn", suckutils.ConcatTwo("Connected to ", addr))

	a.Lock()
	defer a.Unlock()
	a.conns = append(a.conns, recon)
}

func (a *app) doAfterReconnect() error {
	a.l.Debug("Conn", suckutils.ConcatTwo("succesfully reconnected to app \"", string(a.servicename)))
	return nil
}

func (a *app) NewMessage() connector.MessageReader {
	return connector.NewBasicMessage()
}

func (a *app) Handle(msg connector.MessageReader) error {
	appservmessage, err := protocol.DecodeAppMessageToAppServerMessage(msg.(*connector.BasicMessage).Payload)
	if err != nil {
		return err
	}
	cl, err := a.clients.get(appservmessage.ConnectionUID, appservmessage.Generation)
	if err != nil {
		// TODO: send error?
		a.l.Error("Handle/GetClient", err)
		return nil
	}
	if cl != nil {
		appservmessage.ApplicationID = a.appid
		if err = cl.send(appservmessage.EncodeToClientMessage()); err != nil {
			a.l.Error("Handle/Send", err)
			return nil
		}
	}
	// TODO: send disconnection
	return nil
}

func (a *app) HandleClose(reason error) {
	a.l.Warning("Conn", suckutils.ConcatTwo("conn closed, reason err: ", reason.Error()))
}
