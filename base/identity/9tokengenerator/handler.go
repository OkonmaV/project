package main

import (
	"encoding/json"
	"errors"
	"project/base/identity/repo"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/dgrijalva/jwt-go"
)

type Handler struct {
	jwtKey      []byte
	getUserData *httpservice.InnerService
}

func NewHandler(jwtKey string, getuserdata *httpservice.InnerService) (*Handler, error) {
	return &Handler{jwtKey: []byte(jwtKey), getUserData: getuserdata}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	hashLogin := strings.Trim(r.Uri.Path, "/")
	if len(hashLogin) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	getUserDataReq, err := conf.getUserData.CreateRequestFrom(suckhttp.GET, suckutils.ConcatThree("/", hashLogin, "?fields=metaid,role,surname,name"), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	getUserDataReq.AddHeader(suckhttp.Accept, "application/json")
	getUserDataResp, err := conf.getUserData.Send(getUserDataReq)
	if err != nil {
		l.Error("Send", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	if i, t := getUserDataResp.GetStatus(); i != 200 {
		l.Debug("Resp from getuserdata", t)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if len(getUserDataResp.GetBody()) == 0 {
		l.Error("Resp from getuserdata", errors.New("empty body at 200"))
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	var clms repo.CookieClaims
	if err := json.Unmarshal(getUserDataResp.GetBody(), &clms); err != nil {
		l.Error("Unmarshal", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	clms.Login = hashLogin
	//var jwtToken string
	jwtToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &clms).SignedString(conf.jwtKey)
	if err != nil {
		l.Error("Generating new jwtToken", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	resp := suckhttp.NewResponse(200, "OK")
	resp.SetBody([]byte(jwtToken))

	return resp, nil
}
