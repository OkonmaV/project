package main

import (
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type user struct {
	UserId string `bson:"userid"`
	Type   int    `bson:"type"`
	//StartDateTime time.Time `bson:"startdatetime"`
}

type Handler struct {
	mgoColl      *mgo.Collection
	tokenDecoder *httpservice.InnerService
}

func NewHandler(col *mgo.Collection, tokendecoder *httpservice.InnerService) (*Handler, error) {

	return &Handler{mgoColl: col, tokenDecoder: tokendecoder}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.HttpMethod("PATCH") {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	//AUTH
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

	chatId := strings.Trim(r.Uri.Path, "/")
	if chatId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	addUserId := r.Uri.Query().Get("adduserid")
	if addUserId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	query := bson.M{"_id": chatId, "type": 2, "users": bson.M{"$elemMatch": bson.M{"userid": userId, "type": 1}}}

	update := bson.M{"$addToSet": bson.M{"users": &user{UserId: addUserId, Type: 1}}}

	changeInfo, err := conf.mgoColl.UpdateAll(query, update)
	if err != nil {
		return nil, err
	}
	if changeInfo.Matched == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	switch changeInfo.Updated {
	case 1:
		return suckhttp.NewResponse(201, "Created"), nil
	case 0:
		return suckhttp.NewResponse(200, "OK"), nil
	default:
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
}
