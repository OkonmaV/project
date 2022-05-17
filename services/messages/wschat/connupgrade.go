package main

import "project/wsconnector"

// wsservice.Handler {wsconnector.WsHandler {wsconnector.UpgradeReqChecker}} interface implementation
func (wsc *wsconn) CheckPath(path []byte) wsconnector.StatusCode {
	//wsc.l.Debug("path=id", string(path[1:]))
	// rand.Seed(time.Now().Unix())
	// wsc.userId = userid("testuser" + strings.Trim(strings.Replace(fmt.Sprint(rand.Perm(4)), " ", "", -1), "[]"))
	wsc.userId = userid(path[1:])
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
	wsc.l.Debug("New conn", "userid: "+string(wsc.userId))
	return 200
}
