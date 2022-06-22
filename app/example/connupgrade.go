package main

import (
	"fmt"
	"math/rand"
	"project/wsconnector"
	"strings"
	"time"

	"github.com/big-larry/suckutils"
)

// wsservice.Handler {wsconnector.WsHandler {wsconnector.UpgradeReqChecker}} interface implementation
func (wsc *wsconn) CheckPath(path []byte) wsconnector.StatusCode {
	//wsc.l.Debug("path=id", string(path[1:]))
	rand.Seed(time.Now().UnixNano())
	wsc.userId = "testuser-" + strings.Trim(strings.Replace(fmt.Sprint(rand.Perm(4)), " ", "", -1), "[]")
	return 200
}

// wsservice.Handler {wsconnector.WsHandler {wsconnector.UpgradeReqChecker}} interface implementation
func (wsc *wsconn) CheckHost(host []byte) wsconnector.StatusCode {
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
	wsc.l = wsc.l.NewSubLogger(suckutils.ConcatTwo("User-", string(wsc.userId)))

	return 200
}
