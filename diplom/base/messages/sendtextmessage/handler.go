package main

import (
	"net/url"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/roistat/go-clickhouse"
)

type Handler struct {
	mgoColl         *mgo.Collection
	tokenDecoder    *httpservice.InnerService
	clickhouseConn  *clickhouse.Conn
	clickhouseTable string
}

func NewHandler(mgoColl *mgo.Collection, tokendecoder *httpservice.InnerService, clickhouseConn *clickhouse.Conn, clickhouseTable string) (*Handler, error) {

	return &Handler{mgoColl: mgoColl, tokenDecoder: tokendecoder, clickhouseConn: clickhouseConn, clickhouseTable: clickhouseTable}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST {
		l.Debug("Request", "not POST")
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	chatId, err := bson.NewObjectIdFromHex(strings.Trim(r.Uri.Path, "/"))
	if err != nil {
		l.Debug("Request", "chatId (path) is nil or not objectId")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	message := formValues.Get("text")
	if message == "" { // check len?
		l.Debug("Request", "field \"text\" is empty")
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	// AUTH
	koki, ok := r.GetCookie("koki")
	if !ok || len(koki) < 5 {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	tokenDecoderReq, err := conf.tokenDecoder.CreateRequestFrom(suckhttp.GET, suckutils.Concat("/", koki), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	tokenDecoderReq.AddHeader(suckhttp.Accept, "text/plain")
	tokenDecoderResp, err := conf.tokenDecoder.Send(tokenDecoderReq)
	if err != nil {
		l.Error("Send", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if i, t := tokenDecoderResp.GetStatus(); i/100 != 2 {
		l.Debug("Resp from tokendecoder", t)
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	userId := string(tokenDecoderResp.GetBody())
	if userId == "" {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	//

	if err := conf.mgoColl.Find(bson.M{"_id": chatId, "users.userid": userId}).Select(bson.M{"_id": 1}).One(nil); err != nil {
		if err == mgo.ErrNotFound {
			l.Debug("Find", "no chat with that id")
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	query, err := clickhouse.BuildInsert(conf.clickhouseTable,
		clickhouse.Columns{"time", "chatid", "userid", "message", "type"},
		clickhouse.Row{time.Now().Format("2006.01.02 15:04:05"), chatId, userId, message, 1})
	if err != nil {
		return nil, err
	}

	if err = query.Exec(conf.clickhouseConn); err != nil {
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
