package appservice

import (
	"context"
	"errors"
	"project/app/protocol"
	"project/connector"
	"project/logs/logger"
	"sync"
	"time"
)

type Sender interface {
	Send(message []byte) error
}

type appserver struct {
	conn       *connector.EpollConnector
	handlefunc handleFunc
	sendQueue  chan []byte
	l          logger.Logger

	connAlive bool
	sync.RWMutex
}

func newAppService(ctx context.Context, l logger.Logger, sendqueueSize int, handlefunc handleFunc) *appserver {
	as := &appserver{
		handlefunc: handlefunc,
		sendQueue:  make(chan []byte, sendqueueSize),
		l:          l,
	}
	go as.sendWorker(ctx)

	return as
}

type handleFunc func(*protocol.AppMessage) error

func (*appserver) NewMessage() connector.MessageReader {
	return &protocol.AppMessage{}
}

func (as *appserver) Handle(msg connector.MessageReader) error {
	if err := as.handlefunc(msg.(*protocol.AppMessage)); err != nil {
		as.l.Error("Handle", err)
	}
	return nil
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
	for {
		as.RLock()
		if !as.connAlive {
			as.l.Debug("sendWorker", "conn is dead, timeout")
			time.Sleep(time.Second * 2)
			continue
		}
		select {
		case <-ctx.Done():
			as.l.Debug("sendWorker", "context done, exiting")
			// TODO: дамп очереди?
			return
		case message := <-as.sendQueue:
			if err := as.conn.Send(message); err != nil {
				as.l.Error("sendWorker/Send", err)
				// TODO: куда девать достанный из канала message?
			}
		}
		as.RUnlock()
	}
}

func (as *appserver) HandleClose(reason error) {
	as.Lock()
	defer as.Unlock()
	as.connAlive = false
}
