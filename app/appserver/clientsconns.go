package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"project/app/protocol"
	"project/logs/logger"
	"project/wsconnector"
	"strconv"
	"sync"

	"time"

	"github.com/big-larry/suckutils"
)

type clientsConnsList struct {
	conns []client

	sync.RWMutex
}

type client struct {
	connuid      protocol.ConnUID
	curr_gen     byte
	conn         wsconnector.Conn
	closehandler func() error
	apps         *applications

	l logger.Logger
	sync.Mutex
}

const connslist_max_freeconnuid_scan_iterations = 2
const connslist_freeconnuid_scan_timeout = time.Second * 2

func newClientsConnsList(size int, apps *applications) *clientsConnsList {
	if size == 0 || size+1 > protocol.Max_ConnUID {
		panic("clients list impossible size (must be 0<size<protocol.Max_ConnUID-1)")
	}
	cc := &clientsConnsList{conns: make([]client, size+1)}
	for i := 1; i < len(cc.conns); i++ {
		cc.conns[i].connuid = protocol.ConnUID(i)
		cc.conns[i].apps = apps
	}
	return cc
}

func (cc *clientsConnsList) newClient() (*client, error) {
	cc.Lock()
	for iter := 1; iter <= connslist_max_freeconnuid_scan_iterations; iter++ {
		for i := 1; i < len(cc.conns); i++ {
			cc.conns[i].Lock()
			if cc.conns[i].conn == nil {
				cc.conns[i].conn = &wsconnector.EpollWSConnector{}
				cc.conns[i].curr_gen++
				cc.conns[i].Unlock()
				return &cc.conns[i], nil
			}
			cc.conns[i].Unlock()
		}
		cc.Unlock()
		if iter != connslist_max_freeconnuid_scan_iterations {
			time.Sleep(connslist_freeconnuid_scan_timeout) // иначе connuid не освободится из-за мьютекса
			cc.Lock()
		}
	}
	return nil, errors.New("no free permitted connections")
}

func (cc *clientsConnsList) handleCloseClientConn(connuid protocol.ConnUID) error {
	cc.Lock()
	defer cc.Unlock()
	if connuid == 0 || int(connuid) >= len(cc.conns) {
		return errors.New(suckutils.ConcatThree("impossible connuid (must be 0<connuid<=len(cc.conns)): \"", strconv.Itoa(int(connuid)), "\""))
	}
	cc.conns[connuid].Lock()
	cc.conns[connuid].conn = nil
	cc.conns[connuid].Unlock()
	return nil
}

// returns nil client on not found
func (cc *clientsConnsList) get(connuid protocol.ConnUID, generation byte) (*client, error) {
	if connuid == 0 || int(connuid) >= len(cc.conns) {
		return nil, errors.New(suckutils.ConcatThree("impossible connuid (must be 0<connuid<=len(cc.conns)): \"", strconv.Itoa(int(connuid)), "\""))
	}
	cc.RLock()
	cc.conns[connuid].Lock()
	if cc.conns[connuid].conn != nil {
		cc.conns[connuid].Unlock()
		if cc.conns[connuid].curr_gen == generation {
			return &cc.conns[connuid], nil
		}
		return nil, nil
	}
	cc.conns[connuid].Unlock()
	return nil, nil
}

// wsservice.Handler {wsconnector.WsHandler} interface implementation
func (cl *client) Handle(msg interface{}) error {
	//cl.l.Info("NEW MESSAGE", fmt.Sprint(msg))

	cl.Lock()
	ts := time.Now().UnixNano()

	asmessage := msg.(*protocol.AppServerMessage)
	if asmessage.Type == protocol.TypeSettingsReq {
		if asmessage.ApplicationID == 0 {
			hdrs, _ := json.Marshal(struct {
				C string `json:"content-type"`
			}{C: "appsindex"})

			clmsg, _ := protocol.EncodeClientMessage(protocol.TypeSettingsReq, 0, ts, hdrs, cl.apps.appsIndex)

			err := cl.conn.Send(clmsg)
			if err != nil {
				cl.l.Error("Send", err)
			}
			cl.Unlock()
			return err
		} else {
			if body, err := cl.apps.getSettings(asmessage.ApplicationID); err != nil {
				cl.l.Error("GetSettings", err)
				cl.Unlock()
				return err
			} else {
				hdrs, _ := json.Marshal(struct {
					C string `json:"content-type"`
				}{C: "appsettings"})

				clmsg, _ := protocol.EncodeClientMessage(protocol.TypeSettingsReq, 0, ts, hdrs, body)

				err := cl.conn.Send(clmsg)
				if err != nil {
					cl.l.Error("Send", err)
				}
				cl.Unlock()
				return err
			}
		}
	}
	app, err := cl.apps.get(asmessage.ApplicationID)
	if err != nil {
		// TODO: send UpdateSettings?
		cl.l.Error("Handle/Message.ApplicationID", err)
		cl.Unlock()
		return nil
	}

	ts_message := make([]byte, 9)
	ts_message[0] = byte(protocol.TypeTimestamp)
	binary.BigEndian.PutUint64(ts_message[1:], uint64(ts))
	if err := cl.conn.Send(ts_message); err != nil {
		cl.l.Error("Send", err)
		cl.Unlock()
		return err
	}
	cl.Unlock()

	asmessage.Timestamp = ts
	asmessage.ConnectionUID = cl.connuid
	asmessage.Generation = cl.curr_gen
	appmessage, err := asmessage.EncodeToAppMessage()
	if err != nil {
		panic(err)
	}

	app.SendToAll(appmessage)
	// TODO: успешность отправки сообщить клиенту?
	return nil
}

func (cl *client) send(message []byte) error {
	cl.Lock()
	defer cl.Unlock()

	if cl.conn != nil {
		return cl.conn.Send(message)
	} else {
		return wsconnector.ErrNilConn
	}
}

// wsservice.Handler {wsconnector.WsHandler} interface implementation
func (cl *client) HandleClose(err error) {
	cl.l.Debug("Conn", suckutils.ConcatTwo("closed, reason: ", err.Error()))
	// TODO: send disconnection? но ому конкретно? можно всем
	msg, _ := (&protocol.AppServerMessage{Type: protocol.TypeDisconnection, ConnectionUID: cl.connuid, Generation: cl.curr_gen, Timestamp: time.Now().UnixNano()}).EncodeToAppMessage()
	cl.apps.SendToAll(msg)
	if cl.closehandler != nil {
		if err := cl.closehandler(); err != nil {
			cl.l.Error("Conn", errors.New(suckutils.ConcatTwo("error on closehandler, err: ", err.Error())))
		}
	}
}

// wsservice.Handler {wsconnector.WsHandler {wsconnector.UpgradeReqChecker}} interface implementation
func (cl *client) CheckPath(path []byte) wsconnector.StatusCode {
	return 200
}

// wsservice.Handler {wsconnector.WsHandler {wsconnector.UpgradeReqChecker}} interface implementation
func (cl *client) CheckHost(host []byte) wsconnector.StatusCode {
	return 200
}

// wsservice.Handler {wsconnector.WsHandler {wsconnector.UpgradeReqChecker}} interface implementation
func (cl *client) CheckHeader(key []byte, value []byte) wsconnector.StatusCode {
	return 200
}

// wsservice.Handler {wsconnector.WsHandler {wsconnector.UpgradeReqChecker}} interface implementation
func (cl *client) CheckBeforeUpgrade() wsconnector.StatusCode {
	return 200
}

func (cl *client) NewMessage() wsconnector.MessageReader {
	return &protocol.AppServerMessage{}
}