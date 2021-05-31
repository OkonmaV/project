package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
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

	codee, err := strconv.Atoi(r.Uri.Query().Get("code")) //TODO: откуда код?
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	uuid := r.Uri.Query().Get("uuid")
	if uuid == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	// get userData from regcodes table
	var trntlRes []tuple

	if err = conf.trntlConn.SelectTyped(conf.trntlTable, "primary", 0, 1, tarantool.IterEq, []interface{}{code}, &trntlRes); err != nil {
		return nil, err
	}
	if len(trntlRes) == 0 {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	var userData map[string]string
	if err = json.Unmarshal([]byte(trntlRes[0].Data), &userData); err != nil {
		return nil, err
	}

	userMail, ok := userData["_id"]
	if !ok {
		l.Error("Get userHash", errors.New("No hash field founded in tarantool.regcodes"))
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	userMailHashed, err := getMD5(userMail)
	if err != nil {
		l.Error("Getting md5", err)
		return nil, nil
	}
	//

	// emailVerify req
	emailVerifyReq, err := conf.emailVerify.CreateRequestFrom(suckhttp.POST, suckutils.ConcatTwo("/?id=", userMailHashed), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return nil, nil
	}
	emailVerifyReq.Body = []byte(uuid)
	emailVerifyReq.AddHeader(suckhttp.Content_Type, "text/plain")

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
	userPassword, ok := userData["password"]
	if !ok {
		l.Error("Get userPassword", errors.New("No password field founded in tarantool.regcodes"))
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	delete(userData, "password")

	userRegistrationReq, err := conf.userRegistration.CreateRequestFrom(suckhttp.POST, suckutils.ConcatTwo("/?login=", userMailHashed), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return nil, nil
	}
	userRegistrationReq.Body = []byte(userPassword)
	userRegistrationReq.AddHeader(suckhttp.Content_Type, "text/plain")

	userRegistrationResp, err := conf.userRegistration.Send(userRegistrationReq)
	if err != nil {
		l.Error("Send req to userregistration", err)
		return nil, nil
	}
	if i, t := userRegistrationResp.GetStatus(); i != 200 {
		l.Error("Resp from userregistration", errors.New(suckutils.ConcatTwo("statuscode is ", t)))
		return nil, nil
	}
	//

	// setUserData req
	setUserDataReq, err := conf.setUserData.CreateRequestFrom(suckhttp.POST, "", r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return nil, nil // ????????????? not OK?????
	}
	setUserDataReq.AddHeader(suckhttp.Content_Type, "application/json")

	setUserDataReq.Body, err = json.Marshal(userData)
	if err != nil {
		l.Error("Marshalling userData", err)
		return nil, nil // ????????????? not OK?????
	}

	setUserDataResp, err := conf.setUserData.Send(setUserDataReq)
	if err != nil {
		l.Error("Send req to setuserdata", err)
		return nil, nil // ????????????? not OK?????
	}
	if i, t := setUserDataResp.GetStatus(); i != 200 {
		l.Error("Resp from setuserdata", errors.New(suckutils.ConcatTwo("statuscode is ", t)))
		return nil, nil // ????????????? not OK?????
	}

	return suckhttp.NewResponse(200, "OK"), nil
}

func getMD5(str string) (string, error) {
	hash := md5.New()
	_, err := hash.Write([]byte(str))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
