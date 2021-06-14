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

type Handler struct {
	mgoColl      *mgo.Collection
	tokenDecoder *httpservice.InnerService
}

func NewHandler(col *mgo.Collection, tokendecoder *httpservice.InnerService) (*Handler, error) {

	return &Handler{mgoColl: col, tokenDecoder: tokendecoder}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.DELETE {
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

	chatId := r.Uri.Path
	chatId = strings.Trim(chatId, "/")
	if chatId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	deletionUserId := r.Uri.Query().Get("deluserid")
	if deletionUserId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	query := bson.M{"_id": chatId, "type": 2, "users": bson.M{"$elemMatch": bson.M{"userid": userId, "type": 1}}}

	change := mgo.Change{
		Update:    bson.M{"$pull": bson.M{"users": bson.M{"userid": deletionUserId, "type": bson.M{"$gt": 0}}}},
		Upsert:    false,
		ReturnNew: false,
		Remove:    false,
	}

	changeInfo, err := conf.mgoColl.Find(query).Apply(change, nil)
	if err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}
	if changeInfo.Updated == 1 { //всегда =1
		return suckhttp.NewResponse(200, "OK"), nil
	} else {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

}
