package main

import (
	"encoding/json"
	"net/url"
	"project/base/quizes/repo"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type Handler struct {
	mgoColl      *mgo.Collection
	authSet      *httpservice.InnerService
	tokenDecoder *httpservice.InnerService
}

type cookieData struct {
	UserId string `json:"Login"`
	Role   int    `json:"role"`
}

func NewHandler(col *mgo.Collection, authSet *httpservice.InnerService, tokendecoder *httpservice.InnerService) (*Handler, error) {

	return &Handler{mgoColl: col, tokenDecoder: tokendecoder, authSet: authSet}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.PUT || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || len(r.Body) == 0 {
		l.Debug("Request", "not PUT or content-type isnt application/x-www-form-urlencoded or body is empty")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		l.Debug("ParseQuery", err.Error())
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	quizName := formValues.Get("quizname")
	if quizName == "" {
		l.Debug("Form data", "field \"quizname\" is nil")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// check role DECODING A COOKIE
	// TODO: create helper
	koki, ok := r.GetCookie("koki")
	if !ok || len(koki) < 5 {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	tokenDecoderReq, err := conf.tokenDecoder.CreateRequestFrom(suckhttp.GET, suckutils.Concat("/", koki), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	tokenDecoderReq.AddHeader(suckhttp.Accept, "application/json")
	tokenDecoderResp, err := conf.tokenDecoder.Send(tokenDecoderReq)
	if err != nil {
		l.Error("Send", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if i, t := tokenDecoderResp.GetStatus(); i/100 != 2 {
		l.Debug("Resp from tokendecoder", t)
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	if len(tokenDecoderResp.GetBody()) == 0 {
		l.Debug("Resp from tokendecoder", "empty body")
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	userData := &cookieData{}

	if err = json.Unmarshal(tokenDecoderResp.GetBody(), userData); err != nil {
		l.Error("Unmarshal", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if userData.Role < 2 || userData.UserId == "" {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	//

	quizId := bson.NewObjectId()
	//нет проверки по имени

	//set auth
	// TODO: other authorizer?????
	authSetReq, err := conf.authSet.CreateRequestFrom(suckhttp.POST, suckutils.Concat("/", userData.UserId, ".", quizId.Hex(), "?perm=1"), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	authSetResp, err := conf.authSet.Send(authSetReq)
	if err != nil {
		l.Error("Send", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if i, t := authSetResp.GetStatus(); i/100 != 2 {
		l.Debug("Resp from auth", t)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	resp := suckhttp.NewResponse(201, "Created")
	if strings.Contains(r.GetHeader(suckhttp.Accept), "text/plain") {
		resp.SetBody([]byte(quizId.Hex())).AddHeader(suckhttp.Content_Type, "text/plain")
	}
	//mongo insert
	if err = conf.mgoColl.Insert(&repo.Quiz{Id: quizId, Name: quizName, CreatorId: userData.UserId}); err != nil {
		return nil, err
	}

	return resp, nil
}