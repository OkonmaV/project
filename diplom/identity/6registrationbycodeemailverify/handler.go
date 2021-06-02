package main

import (
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
	verify           *httpservice.InnerService
	userRegistration *httpservice.InnerService
	setUserData      *httpservice.InnerService
}

type tuple struct {
	Code     int
	Data     string
	MetaId   string
	Surname  string
	Name     string
	Hash     string
	Password string
	Status   int
}

func (handler *CreateVerifyEmail) Close() error {
	return handler.trntlConn.Close()
}

func NewCreateVerifyEmail(trntlAddr string, trntlTable string, verify *httpservice.InnerService, userRegistration *httpservice.InnerService, setUserData *httpservice.InnerService) (*CreateVerifyEmail, error) {

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
	return &CreateVerifyEmail{trntlConn: trntlConn, trntlTable: trntlTable, verify: verify, userRegistration: userRegistration, setUserData: setUserData}, nil
}

func (conf *CreateVerifyEmail) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST { //POST ????
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	hash, err := strconv.Atoi(r.Uri.Query().Get("hash"))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	uuid := r.Uri.Query().Get("uuid")
	if uuid == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// get userData from regcodes table
	var trntlRes []tuple

	if err = conf.trntlConn.SelectTyped(conf.trntlTable, "secondary", 0, 1, tarantool.IterEq, []interface{}{hash}, &trntlRes); err != nil {
		return nil, err
	}
	if len(trntlRes) == 0 {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	var userData map[string]interface{}
	if err = json.Unmarshal([]byte(trntlRes[0].Data), &userData); err != nil {
		l.Error("Unmarshalling userData", err)
		return nil, nil
	}

	userMailHashed := trntlRes[0].Hash
	if userMailHashed == "" {
		l.Error("Getting hash from regcodes", errors.New("hash is nil"))
		return nil, nil
	}
	//

	// verify req
	verifyReq, err := conf.verify.CreateRequestFrom(suckhttp.POST, userMailHashed, r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return nil, nil
	}
	verifyReq.AddHeader(suckhttp.Content_Type, "text/plain")
	verifyReq.Body = []byte(uuid)
	verifyResp, err := conf.verify.Send(verifyReq)
	if err != nil {
		return nil, err
	}
	if i, t := verifyResp.GetStatus(); i != 200 {
		if i == 403 {
			return verifyResp, nil
		}
		l.Debug("Responce from verify", t)
		return nil, nil
	}
	//

	// userRegistration req
	userPassword := trntlRes[0].Password
	if userPassword == "" {
		l.Error("Getting password from regcodes", errors.New("password is nil"))
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	userRegistrationReq, err := conf.userRegistration.CreateRequestFrom(suckhttp.PUT, userMailHashed, r)
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
	userData["newlogin"] = trntlRes[0].Hash
	if userPassword == "" {
		l.Error("Getting hash from regcodes", errors.New("hash is nil"))
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	userData["metaid"] = trntlRes[0].MetaId
	if userPassword == "" {
		l.Error("Getting metaid from regcodes", errors.New("metaid is nil"))
		return nil, nil
	}

	setUserDataReq, err := conf.setUserData.CreateRequestFrom(suckhttp.HttpMethod("PATCH"), userMailHashed, r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return nil, nil
	}
	setUserDataReq.AddHeader(suckhttp.Content_Type, "application/json")

	if setUserDataReq.Body, err = json.Marshal(userData); err != nil {
		l.Error("Marshalling userData", err)
		return nil, nil
	}

	setUserDataResp, err := conf.setUserData.Send(setUserDataReq)
	if err != nil {
		l.Error("Send req to setuserdata", err)
		return nil, nil
	}
	if i, t := setUserDataResp.GetStatus(); i != 200 {
		l.Error("Resp from setuserdata", errors.New(suckutils.ConcatTwo("statuscode is ", t)))
		return nil, nil
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
