package identityserver

import (
	"context"
	"encoding/json"
	"errors"
	"project/app/protocol"
	"project/connector"
	"project/logs/logger"
	"strconv"
	"sync"
	"time"

	"github.com/big-larry/suckutils"
)

type Sender interface {
	Send(message []byte) error
}

type appserver struct {
	conn    *connector.EpollConnector
	handler clientHandler

	sendQueue    chan []byte
	backingQueue chan []byte

	l logger.Logger

	connAlive bool
	sync.RWMutex
}

// type handleFunc func(*protocol.IdentityServerMessage_Headers, *protocol.AppMessage) error

// type handleClientAuthReq func(appid, login, password string) (grant string)
// type handleTokenReq func(appid, grant, secret string) (accessttoken, refreshtoken, clientid string)
// type handleAppAuth func(appname, appid string) error
// type handleAppRegistrationReq func(appname string) (appid, secret string)
// type handleClientRegistrationReq func(login, password string)

const backingQueue_size = 1

func newAppService(ctx context.Context, l logger.Logger, sendqueueSize int, handler clientHandler) *appserver {
	as := &appserver{
		handler:      handler,
		sendQueue:    make(chan []byte, sendqueueSize),
		backingQueue: make(chan []byte, backingQueue_size),
		l:            l,
	}
	go as.sendWorker(ctx)

	return as
}

func (*appserver) NewMessage() connector.MessageReader {
	return &protocol.AppMessage{}
}

func (as *appserver) Handle(msg interface{}) (err error) {
	message := msg.(*protocol.AppMessage)

	response := &protocol.AppMessage{ConnectionUID: message.ConnectionUID, Timestamp: message.Timestamp}
	switch message.Type {
	case protocol.TypeAuthData:
		hdrs := &protocol.IdentityServerMessage_Headers{}
		if len(message.Headers) != 0 {
			if err = json.Unmarshal(message.Headers, hdrs); err != nil {
				as.l.Error("Handle/Unmarshal", errors.New("empty headers"))
				goto bad_req
			}
			if len(hdrs.App_Id) == 0 {
				goto bad_req
			}
		} else {
			as.l.Error("Handle", errors.New("empty headers"))
			goto bad_req
		}
		if len(message.Body) > 2 {
			if len(message.Body) >= 1+int(message.Body[0]) {
				if len(message.Body) == 2+int(message.Body[0])+int(message.Body[int(message.Body[0])+1]) {
					if grant, errCode := as.handler.Handle_ClientAuth(hdrs.App_Id, string(message.Body[1:1+int(message.Body[0])]), string(message.Body[2+int(message.Body[0]):])); errCode == 0 {
						if message.Headers, err = json.Marshal(protocol.IdentityServerMessage_Headers{Grant: grant}); err != nil {
							response.Type = protocol.TypeError
							response.Body = []byte{byte(protocol.ErrCodeInternalServerError)}
						} else {
							response.Type = protocol.TypeGrant
						}
						goto sending
					}
				}
			}
		}
	case protocol.TypeRegistration:

	}
bad_req:
	response.Type = protocol.TypeError
	response.Body = []byte{byte(protocol.ErrCodeBadRequest)}
sending:
}

func (as *appserver) Send(message []byte) error {
	// TODO: add timeout?
	if len(message) < protocol.App_message_head_len {
		return errors.New("message len does not satisfy minimal length")
	}
	as.sendQueue <- message
	return nil
}

func (as *appserver) sendWorker(ctx context.Context) {
send_loop:
	for {
		select {
		case message := <-as.backingQueue:
			as.l.Debug("sendWorker", "retrying to send a message")
			if err := as.conn.Send(message); err != nil {
				as.l.Error("sendWorker/Send", err)
				as.backingQueue <- message

				as.RLock()
				if !as.connAlive {
					as.l.Debug("sendWorker", "conn to appserver is dead, timeout")
					as.RUnlock()
					time.Sleep(time.Second * 5)
					continue send_loop
				}
				as.RUnlock()
			}
		case <-ctx.Done():
			break send_loop
		default:
			select {
			case message := <-as.sendQueue:
				as.l.Debug("sendWorker", "sending a message")
				if err := as.conn.Send(message); err != nil {
					as.l.Error("sendWorker/Send", err)
					as.backingQueue <- message
				}
			case <-ctx.Done():
				break send_loop
			}
		}
	}
	as.l.Debug("sendWorker", "context done, send loop terminated")

	//TODO: queue dump?
	var unsended_messages int
	for range as.backingQueue {
		unsended_messages++
	}

	for range as.sendQueue {
		unsended_messages++
	}
	as.l.Debug("sendWorker", suckutils.ConcatTwo("exiting, unsended messages: ", strconv.Itoa(unsended_messages)))
}

func (as *appserver) HandleClose(reason error) {
	if reason != nil {
		as.l.Warning("Conn", suckutils.ConcatTwo("closed, reason: ", reason.Error()))
	} else {
		as.l.Warning("Conn", "closed, no reason specified")
	}

	as.Lock()
	defer as.Unlock()
	as.connAlive = false
}
