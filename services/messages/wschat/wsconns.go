package main

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"project/logs/logger"
	"project/services/messages/messagestypes"

	"project/wsconnector"
	"project/wsservice"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

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

// type cleanReq struct {
// 	uc  *userconns
// 	wsc *wsconn
// }

// // иначе при рассылке сообщений будет дедлок, ибо не получилось отправить сообщение = коннект закрывается
// func connsCleanWorker(ctx context.Context, ch <-chan cleanReq) {
// 	for {
// 		select {
// 		case cr := <-ch:
// 			cr.uc.deleteconn(cr.wsc)
// 		case <-ctx.Done():
// 			return
// 		}
// 	}
// }

// single ws connection

type wsconn struct {
	userId userid
	conn   wsconnector.Sender

	srvc *service
	l    logger.Logger
}

// wsservice.Handler interface implementation
func (wsc *wsconn) HandleNewConnection(conn wsservice.WSconn) error {
	wsc.srvc.adduser(wsc)
	wsc.conn = conn
	return nil
}

// wsservice.Handler {wsconnector.WsHandler} interface implementation
func (wsc *wsconn) Handle(msg interface{}) error {
	//wsc.l.Info("NEW MESSAGE", string(message))

	m := msg.(*message)

	if m.ErrCode != 0 {
		return errors.New(suckutils.ConcatTwo("recieved an errorcode ", strconv.Itoa(m.ErrCode)))
	}

	var has_rights bool

	has_rights = strings.Contains("user1 user2 user3", string(wsc.userId)) /////////////////////SKIP CONFIRMATION VIA MONGODB OR SOME OTHER SHIT

	if !has_rights {
		m.ErrCode = 403
		if enc, err := wsconnector.EncodeJson(m); err != nil {
			panic(err)
		} else {
			return wsc.conn.Send(enc)
		}
	}

	var chat_users []userid

	chat_users = append(chat_users, "user1", "user2", "user3") ////////////////////////////// SKIP GETTING LIST OF CHAT USERS

	var err error
	var errmsg []byte
switching:
	switch m.Type {
	case messagestypes.Text:
		if len(m.Data) == 0 {
			wsc.l.Error("Handle", errors.New("empty message data"))
			return nil
		}

		if err = wsc.srvc.chconn.Insert(suckutils.Concat("VALUES ('", string(m.UserId), "','", m.ChatId, "',", strconv.Itoa(int(m.Type)), ",", formatByteSlice(m.Data), ",now())")); err != nil {
			m.ErrCode = 500
			wsc.l.Error("Insert", err)
			break switching
		}

	case messagestypes.Image:
		img, _, err := image.Decode(bytes.NewReader(m.Data))
		if err != nil {
			m.ErrCode = 500
			wsc.l.Error("image.Decode", err)
			break switching
		}
		var filepath string

		switch mimeFromIncipit(m.Data) {
		case "jpeg":
			filepath = suckutils.Concat(wsc.srvc.path, "/", m.ChatId, strconv.FormatInt(time.Now().UnixMicro(), 10), "-", string(wsc.userId), ".jpeg")
			file, err := os.Create(filepath)
			if err != nil {
				file.Close()
				m.ErrCode = 500
				wsc.l.Error("os.Create", err)
				break switching
			}
			if err = jpeg.Encode(file, img, nil); err != nil {
				file.Close()
				m.ErrCode = 400
				wsc.l.Error("jpeg.Encode", err)
				break switching
			}
			file.Close()
		case "png":
			filepath = suckutils.Concat(wsc.srvc.path, "/", m.ChatId, strconv.FormatInt(time.Now().UnixMicro(), 10), "-", string(wsc.userId), ".png")
			file, err := os.Create(filepath)
			if err != nil {
				file.Close()
				m.ErrCode = 500
				wsc.l.Error("os.Create", err)
				break switching
			}
			if err = png.Encode(file, img); err != nil {
				file.Close()
				m.ErrCode = 500
				wsc.l.Error("png.Encode", err)
				break switching
			}
			file.Close()
		default:
			m.ErrCode = 415
			break switching
		}

		if err = wsc.srvc.chconn.Insert(suckutils.Concat("VALUES ('", string(m.UserId), "','", m.ChatId, "',", strconv.Itoa(int(m.Type)), ",", formatByteSlice([]byte(filepath)), ",now())")); err != nil {
			// if err = os.Remove(filepath); err != nil {
			// 	wsc.l.Error("Remove", err)
			// }
			wsc.l.Error("Insert", err)
			m.ErrCode = 500
		}
		goto success

	default:
		m.ErrCode = 415
	}

	errmsg, err = wsconnector.EncodeJson(m)
	if err != nil {
		panic(err)
	}
	return wsc.conn.Send(errmsg)

success:

	m.Time = time.Now() // SET TIME ONLY AFTER WRITING TO DB
	m.UserId = wsc.userId

	confirmed_msg, err := wsconnector.EncodeJson(m)
	if err != nil {
		panic(err)
	}

	wsc.srvc.sendToMany(confirmed_msg, chat_users)

	return nil
}

func (srvc *service) sendToMany(msg []byte, usersids []userid) {
	var err error
	srvc.RLock()
	defer srvc.RUnlock()

	for _, id := range usersids {
		if conns, ok := srvc.users[id]; ok {
			conns.RLock()
		loop:
			for _, wsc := range conns.conns {
				for i := 1; i < numOfSendMsgTries; i++ {
					if err = wsc.conn.Send(msg); err == nil {
						continue loop
					} else {
						wsc.l.Error("Send", err)
					}
				}
			}
			conns.RUnlock()
		}
	}
}

// wsservice.Handler {wsconnector.WsHandler} interface implementation
func (wsc *wsconn) HandleClose(err error) {
	wsc.l.Error("oh shit", err)
	wsc.srvc.users[wsc.userId].deleteconn(wsc)
}

func formatByteSlice(s []byte) string {
	n := (len(s) * 4) + 1

	result := append(make([]byte, 0, n), "["...)
	for i := 0; i < len(s); i++ {
		result = append(append(result, strconv.Itoa(int(s[i]))...), ","...)
		//j += copy(result[j:], elems[i])
	}
	result = append(result[:len(result)-1], "]"...)
	return *(*string)(unsafe.Pointer(&result))
}

// TODO: заменить на что-то посерьезнее?
// image formats and magic numbers
var magicTable = map[string]string{
	"\xff\xd8\xff":      "jpeg",
	"\x89PNG\r\n\x1a\n": "png",
	"GIF87a":            "gif",
	"GIF89a":            "gif",
}

// mimeFromIncipit returns the mime type of an image file from its first few
// bytes or the empty string if the file does not look like a known file type
func mimeFromIncipit(incipit []byte) string {
	incipitStr := string(incipit)
	for magic, mime := range magicTable {
		if strings.HasPrefix(incipitStr, magic) {
			return mime
		}
	}

	return ""
}
