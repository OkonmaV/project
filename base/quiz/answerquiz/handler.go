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
)

type Handler struct {
	mgoColl       *mgo.Collection
	mgoCollQuizes *mgo.Collection
	//auth       *httpservice.Authorizer
	tokenDecoder *httpservice.InnerService
}

//quiz
type quiz struct {
	Questions map[string]question `bson:"questions"`
}

type question struct {
	Type     int               `bson:"question_type"`
	Position int               `bson:"question_position"`
	Text     string            `bson:"question_text"`
	Answers  map[string]string `bson:"question_answers"`
}

//

//results
type results struct {
	Id       bson.ObjectId       `bson:"_id" json:"id"`
	QuizId   string              `bson:"quizid" json:"quizid"`
	EntityId string              `bson:"entityid" json:"entityid"`
	UserId   string              `bson:"userid" json:"userid"`
	Answers  map[string][]string `bson:"answers" json:"answers"`
	Datetime time.Time           `bson:"datetime" json:"datetime"`
}

//

func NewHandler(col *mgo.Collection, colQ *mgo.Collection, tokendecoder *httpservice.InnerService) (*Handler, error) {

	return &Handler{mgoColl: col, mgoCollQuizes: colQ, tokenDecoder: tokendecoder}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	quizId, err := bson.NewObjectIdFromHex(strings.Trim(r.Uri.Path, "/"))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	values, err := url.ParseQuery(string(r.Body))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	entityId := values.Get("entityid")
	if entityId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	//AUTH

	// k, userId, err := conf.auth.GetAccess(r, l, quizId.Hex(), 1)
	// if err != nil {
	// 	return nil, err
	// }
	// if !k {
	// 	return suckhttp.NewResponse(403, "Forbidden"), nil
	// }
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
	//
	var result results

	if result.UserId = string(tokenDecoderResp.GetBody()); len(result.UserId) == 0 {
		l.Debug("Resp from tokendecoder", "empty body")
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	//check quiz
	var mgoRes quiz
	if err = conf.mgoCollQuizes.FindId(quizId).Select(bson.M{"questions": 1}).One(&mgoRes); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), err
		}
		return nil, err
	}
	//
	result.Answers = make(map[string][]string)
	delete(values, "entityid")
	for questionId, answers := range values {
		if _, ok := mgoRes.Questions[questionId]; !ok {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		result.Answers[questionId] = answers

	}

	result.Datetime = time.Now()

	update := bson.M{"$set": bson.M{"answers": &result.Answers, "entityid": entityId, "quizid": quizId.Hex(), "datetime": result.Datetime, "userid": result.UserId}}
	if _, err = conf.mgoColl.Upsert(bson.M{"quizid": quizId.Hex(), "entityid": entityId, "userid": result.UserId}, update); err != nil {
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
