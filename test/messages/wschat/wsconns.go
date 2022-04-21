package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"project/test/types"
	"project/test/wsconnector"
	"strings"
	"sync"
	"time"
)

const userconns_softlimit int = 3

type userconns struct {
	conns []*wsconn
	sync.RWMutex
}

func createuserconns() *userconns {
	return &userconns{
		conns: make([]*wsconn, 0, userconns_softlimit),
	}
}

func (uc *userconns) addconn(conn *wsconn) {
	uc.Lock()
	defer uc.Unlock()

	uc.conns = append(uc.conns, conn)
}

func (uc *userconns) deleteconn(conn *wsconn) {
	uc.Lock()
	defer uc.Unlock()
	for i := 0; i < len(uc.conns); i++ {
		if uc.conns[i] == conn {
			uc.conns = append(uc.conns[:i], uc.conns[i+1:]...)
			break
		}
	}
	if cap(uc.conns) > userconns_softlimit && len(uc.conns) < userconns_softlimit {
		b := uc.conns
		uc.conns = make([]*wsconn, len(b), userconns_softlimit)
		copy(uc.conns, b)
	}
}

type wsconn struct {
	userId userid
	conn   wsconnector.Sender

	srvc *service
	l    types.Logger
}

func (wsc *wsconn) CheckPath(path []byte) wsconnector.StatusCode {
	wsc.l.Debug("path", string(path))
	return 200
}
func (wsc *wsconn) CheckHost(host []byte) wsconnector.StatusCode {
	wsc.l.Debug("host", string(host))
	return 200
}

func (wsc *wsconn) CheckHeader(key []byte, value []byte) wsconnector.StatusCode {
	// if string(key)!="Cookie"{
	// 	return 200
	// }
	rand.Seed(time.Now().Unix())
	wsc.userId = userid("testuser" + strings.Trim(strings.Replace(fmt.Sprint(rand.Perm(4)), " ", "", -1), "[]"))
	return 200
}

func (wsc *wsconn) CheckBeforeUpgrade() wsconnector.StatusCode {
	if len(wsc.userId) == 0 {
		return 403
	}
	return 200
}

func (wsc *wsconn) HandleWSCreating(sender wsconnector.Sender) error {
	wsc.srvc.adduser(wsc)
	wsc.conn = sender
	return nil
}

func (wsc *wsconn) Handle(message []byte) error {
	wsc.l.Info("NEW MESSAGE", string(message))

	wsc.srvc.RLock()
	defer wsc.srvc.RUnlock()

	for id, uc := range wsc.srvc.users {
		uc.RLock()
		for _, ws := range uc.conns {
			ws.conn.Send(bytes.Join([][]byte{[]byte("new message for " + id), message}, []byte(" : ")))
		}
		uc.RUnlock()
	}
	return nil
}

func (wsc *wsconn) HandleClose(err error) {
	wsc.l.Error("oh shit", err)
	wsc.srvc.users[wsc.userId].deleteconn(wsc)
}
