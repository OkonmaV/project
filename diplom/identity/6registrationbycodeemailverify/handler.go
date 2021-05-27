package main

import (
	"encoding/json"
	"strconv"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/suckhttp"
	"github.com/tarantool/go-tarantool"
)

type CreateVerifyEmail struct {
	trntlConn        *tarantool.Connection
	trntlTable       string
	emailVerify      *httpservice.InnerService
	userRegistration *httpservice.InnerService
	setUserData      *httpservice.InnerService
}

type tuple struct {
	Code   int
	Data   string
	MetaId int
}

func (handler *CreateVerifyEmail) Close() error {
	return handler.trntlConn.Close()
}

func NewCreateVerifyEmail(trntlAddr string, trntlTable string, emailVerify *httpservice.InnerService, userRegistration *httpservice.InnerService, setUserData *httpservice.InnerService) (*CreateVerifyEmail, error) {

	trntlConn, err := tarantool.Connect(trntlAddr, tarantool.Opts{
		// User: ,
		// Pass: ,
		Timeout:       500 * time.Millisecond,
		Reconnect:     1 * time.Second,
		MaxReconnects: 4,
	})
	if err != nil {
		return nil, err
	}
	logger.Info("Tarantool", "Connected!")
	return &CreateVerifyEmail{trntlConn: trntlConn, trntlTable: trntlTable, emailVerify: emailVerify, userRegistration: userRegistration, setUserData: setUserData}, nil
}

func (conf *CreateVerifyEmail) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	code, err := strconv.Atoi(r.Uri.Query().Get("code"))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), err
	}
	uuid := r.Uri.Query().Get("uuid")
	if uuid == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	// get userData from regcodes table
	var trntlRes []tuple
	err = conf.trntlConn.SelectTyped(conf.trntlTable, "primary", 0, 1, tarantool.IterEq, []interface{}{code}, &trntlRes)
	if err != nil {
		return nil, err
	}
	if len(trntlRes) == 0 {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	var userData map[string]interface{}
	err = json.Unmarshal([]byte(trntlRes[0].Data), &userData)
	if err != nil {
		return nil, err
	}
	delete(userData, "password")
	//
	// emailVerify req
	emailVerifyReq, err := conf.emailVerify.CreateRequestFrom(suckhttp.POST, "", r)
	if err != nil {
		return nil, err
	}
	emailVerifyReq.AddHeader(suckhttp.Content_Type, "application/json")
	emailVerifyReqInfo := make(map[string]interface{}, 2)
	emailVerifyReqInfo["code"] = code
	emailVerifyReqInfo["uuid"] = uuid
	emailVerifyReq.Body, err = json.Marshal(emailVerifyReqInfo)
	if err != nil {
		return nil, err
	}

	emailVerifyResp, err := conf.emailVerify.Send(emailVerifyReq)
	if err != nil {
		return nil, err
	}
	if i, t := emailVerifyResp.GetStatus(); i != 200 {
		if i == 403 {
			return emailVerifyResp, nil
		}
		l.Debug("Responce from emailVerify", t)
		return nil, nil
	}
	//
	// userRegistration req
	userRegistrationReq, err := conf.userRegistration.CreateRequestFrom(suckhttp.POST, "", r)
	if err != nil {
		return nil, err
	}
	userRegistrationReq.AddHeader(suckhttp.Content_Type, "application/json")

	userRegistrationReqInfo := make(map[string]interface{}, 2)
	var ok bool
	userRegistrationReqInfo["hash"], ok = userData["_id"]
	if !ok {
		l.Debug("Get userHash", "No hash field founded in tarantool.regcodes")
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	userRegistrationReqInfo["password"], ok = userData["password"]
	if !ok {
		l.Debug("Get userPassword", "No password field founded in tarantool.regcodes")
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	userRegistrationReq.Body, err = json.Marshal(userRegistrationReqInfo)
	if err != nil {
		return nil, err
	}

	userRegistrationResp, err := conf.userRegistration.Send(userRegistrationReq)
	if err != nil {
		return nil, err
	}
	if i, t := userRegistrationResp.GetStatus(); i != 200 {
		if i == 403 {
			return emailVerifyResp, nil
		}
		l.Debug("Responce from userRegistration", t)
		return nil, nil
	}
	//
	// setUserData req
	setUserDataReq, err := conf.setUserData.CreateRequestFrom(suckhttp.POST, "", r)
	if err != nil {
		return nil, err
	}
	setUserDataReq.AddHeader(suckhttp.Content_Type, "application/json")

	setUserDataReq.Body, err = json.Marshal(userData)
	if err != nil {
		return nil, err
	}

	setUserDataResp, err := conf.setUserData.Send(setUserDataReq)
	if err != nil {
		return nil, err // ????????????? not OK?????
	}
	if i, t := setUserDataResp.GetStatus(); i != 200 {
		if i == 403 {
			return emailVerifyResp, nil
		}
		l.Debug("Responce from setUserData", t)
		return nil, nil // ????????????? not OK?????
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
