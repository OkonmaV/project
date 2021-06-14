package main

import (
	"errors"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type chat struct {
	Id    string `bson:"_id"`
	Type  int    `bson:"type"`
	Users []user `bson:"users"`
	Name  string `bson:"name,omitempty"`
}
type user struct {
	UserId        string    `bson:"userid"`
	Type          int       `bson:"type"`
	StartDateTime time.Time `bson:"startdatetime"`
	EndDateTime   time.Time `bson:"enddatetime"`
}

type Handler struct {
	tokenDecoder      *httpservice.InnerService
	mgoColl           *mgo.Collection
	mgoCollForDeleted *mgo.Collection
}

func NewHandler(col *mgo.Collection, colDel *mgo.Collection, tokendecoder *httpservice.InnerService) (*Handler, error) {

	return &Handler{mgoColl: col, tokenDecoder: tokendecoder, mgoCollForDeleted: colDel}, nil
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

	//select
	query := bson.M{"_id": chatId, "users": bson.M{"$elemMatch": bson.M{"userid": userId, "type": 1}}}

	change := mgo.Change{
		Update:    bson.M{"$set": bson.M{"deletion": 1}},
		Upsert:    true,
		ReturnNew: false,
		Remove:    false,
	}

	if _, err := conf.mgoColl.Find(query).Apply(change, nil); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	//upsert
	if _, err := conf.mgoCollForDeleted.UpsertId(chatId, nil); err != nil {
		return nil, err
	}

	//delete
	if err := conf.mgoColl.RemoveId(chatId); err != nil {
		if err == mgo.ErrNotFound {
			l.Error("Removing from chats", errors.New("trying remove already removed chat"))
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}
	return suckhttp.NewResponse(200, "OK"), nil

}
