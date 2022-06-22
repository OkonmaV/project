package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"net/url"
	"project/base/identity/repo"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/tarantool/go-tarantool"
)

type Handler struct {
	trntlConn      *tarantool.Connection
	trntlTable     string
	tokenGenerator *httpservice.InnerService
}

func (handler *Handler) Close() error {
	return handler.trntlConn.Close()
}

func NewHandler(trntlAddr string, trntlTable string, tokenGenerator *httpservice.InnerService) (*Handler, error) {

	trntlConn, err := repo.ConnectToTarantool(trntlAddr)
	if err != nil {
		return nil, err
	}
	logger.Info("Tarantool", "Connected!")
	return &Handler{trntlConn: trntlConn, trntlTable: trntlTable, tokenGenerator: tokenGenerator}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || r.GetMethod() != suckhttp.POST {
		l.Debug("Request", "not POST or wrong content-type")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	formValue, err := url.ParseQuery(string(r.Body))
	if err != nil {
		l.Error("Parsing r.Body", err)
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	login := formValue.Get("login")
	password := formValue.Get("password")
	if login == "" || password == "" {
		l.Debug("Request", "login or password not specified in body")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	hashLogin, err := getMD5(login)
	if err != nil {
		l.Error("Getting md5", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	hashPassword, err := getMD5(password)
	if err != nil {
		l.Error("Getting md5", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	var trntlAuthRec []repo.TarantoolAuthTuple
	if err = conf.trntlConn.SelectTyped(conf.trntlTable, "secondary", 0, 1, tarantool.IterEq, []interface{}{hashLogin, hashPassword}, &trntlAuthRec); err != nil {
		return nil, err
	}
	if len(trntlAuthRec) == 0 {
		l.Debug("Tarantool Select", "pair login+pass not found")
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	// tokengenerator req
	tokenReq, err := conf.tokenGenerator.CreateRequestFrom(suckhttp.GET, suckutils.ConcatTwo("/", trntlAuthRec[0].UserId), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	tokenReq.Body = []byte(hashLogin)
	tokenResp, err := conf.tokenGenerator.Send(tokenReq)
	if err != nil {
		l.Error("Send req to tokengenerator", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if i, t := tokenResp.GetStatus(); i/100 != 2 {
		l.Error("Resp from tokengenerator", errors.New(suckutils.ConcatTwo("statuscode is ", t)))
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	if len(tokenResp.GetBody()) == 0 {
		l.Error("Resp from tokengenerator", errors.New("body is empty"))
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	//

	expires := time.Now().Add(24 * time.Hour).String()
	resp := suckhttp.NewResponse(302, "Found")
	resp.SetHeader(suckhttp.Location, "/")
	resp.SetHeader(suckhttp.Set_Cookie, suckutils.ConcatFour("koki=", string(tokenResp.GetBody()), ";Expires=", expires))

	return resp, nil
}

func getMD5(str string) (string, error) {
	hash := md5.New()
	_, err := hash.Write([]byte(str))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
