package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"project/test/messages/messagestypes"
	"project/test/types"
	"project/test/wsconnector"
	"strconv"
	"sync"
	"time"

	"github.com/big-larry/suckutils"
)

// all user's ws connections
type userconns struct {
	conns []*wsconn
	sync.RWMutex
}

const userconns_softlimit int = 3

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

// single ws connection

type wsconn struct {
	userId userid
	conn   wsconnector.Sender

	srvc *service
	l    types.Logger
}

// wsservice.Handler interface implementation
func (wsc *wsconn) HandleWSCreating(sender wsconnector.Sender) error {
	wsc.srvc.adduser(wsc)
	wsc.conn = sender
	return nil
}

// wsservice.Handler {wsconnector.WsHandler {wsconnector.UpgradeReqChecker}} interface implementation
func (wsc *wsconn) CheckPath(path []byte) wsconnector.StatusCode {
	wsc.l.Debug("path=id", string(path[1:]))
	// rand.Seed(time.Now().Unix())
	// wsc.userId = userid("testuser" + strings.Trim(strings.Replace(fmt.Sprint(rand.Perm(4)), " ", "", -1), "[]"))
	wsc.userId = userid(path[1:])
	return 200
}

// wsservice.Handler {wsconnector.WsHandler {wsconnector.UpgradeReqChecker}} interface implementation
func (wsc *wsconn) CheckHost(host []byte) wsconnector.StatusCode {
	wsc.l.Debug("host", string(host))
	return 200
}

// wsservice.Handler {wsconnector.WsHandler {wsconnector.UpgradeReqChecker}} interface implementation
func (wsc *wsconn) CheckHeader(key []byte, value []byte) wsconnector.StatusCode {
	// if string(key)!="Cookie"{
	// 	return 200
	// }
	return 200
}

// wsservice.Handler {wsconnector.WsHandler {wsconnector.UpgradeReqChecker}} interface implementation
func (wsc *wsconn) CheckBeforeUpgrade() wsconnector.StatusCode {
	if len(wsc.userId) == 0 {
		return 403
	}
	return 200
}

// wsservice.Handler {wsconnector.WsHandler} interface implementation
func (wsc *wsconn) Handle(msg wsconnector.MessageReader) error {
	//wsc.l.Info("NEW MESSAGE", string(message))

	m := msg.(*message)

	// SKIP CONFIRMATION VIA MONGODB OR SOME OTHER SHIT

	switch m.Type {
	case messagestypes.Text:
		if len(m.Data) == 0 {
			return errors.New("empty message data")
		}
		if err := wsc.srvc.chconn.Insert(fmt.Sprintf("'%s','%s',%s,'%s',%s", wsc.userId, m.ChatId, messagestypes.Text.String(), string(m.Data), "now()")); err != nil {
			return err
		}
	case messagestypes.Image:
		img, typ, err := image.Decode(bytes.NewReader(m.Data))
		if err != nil {
			return err
		}

		filepath := suckutils.Concat(wsc.srvc.path, "/", m.ChatId, strconv.FormatInt(time.Now().UnixMicro(), 10), "-", string(wsc.userId), ".", typ)
		file, err := os.Create(filepath)
		if err != nil {
			return err
		}
		if err = png.Encode(file, img); err != nil {
			file.Close()
			return err
		}
		file.Close()

		if err := wsc.srvc.chconn.Insert(fmt.Sprintf("'%s','%s',%s,'%s',%s", wsc.userId, m.ChatId, messagestypes.Image.String(), filepath, "now()")); err != nil {
			// if err = os.Remove(filepath); err != nil {
			// 	wsc.l.Error("Remove", err)
			// }
			return err
		}

	default:
		return errors.New("unsupported message content type")
	}

	confirmed_msg, err := wsconnector.EncodeJson(m)
	if err != nil {
		return err
	}

	wsc.srvc.RLock()
	defer wsc.srvc.RUnlock()

	m.Time = time.Now() // SET TIME ONLY AFTER WRITING TO DB
	m.UserId = string(wsc.userId)

	for _, uc := range wsc.srvc.users {
		uc.RLock()
		for _, ws := range uc.conns {
			ws.conn.Send(confirmed_msg)
		}
		uc.RUnlock()
	}

	return nil
}

// wsservice.Handler {wsconnector.WsHandler} interface implementation
func (wsc *wsconn) HandleClose(err error) {
	wsc.l.Error("oh shit", err)
	wsc.srvc.users[wsc.userId].deleteconn(wsc)
}
