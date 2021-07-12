package main

import (
	"net/url"
	"project/base/quizes/repo"
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

func NewHandler(col *mgo.Collection, colQ *mgo.Collection, tokendecoder *httpservice.InnerService) (*Handler, error) {

	return &Handler{mgoColl: col, mgoCollQuizes: colQ, tokenDecoder: tokendecoder}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || len(r.Body) == 0 {
		l.Debug("Request", "not POST or content-type isnt application/x-www-form-urlencoded or body is empty")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	quizId, err := bson.NewObjectIdFromHex(strings.Trim(r.Uri.Path, "/"))
	if err != nil {
		l.Debug("NewObjectIdFromHex", suckutils.ConcatTwo("quizId isnt objectId, error: ", err.Error()))
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	entityId := strings.TrimSpace(r.Uri.Query().Get("entityid"))

	values, err := url.ParseQuery(string(r.Body))
	if err != nil {
		l.Debug("ParseQuery", err.Error())
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var result repo.Results

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

	if result.UserId = string(tokenDecoderResp.GetBody()); len(result.UserId) == 0 {
		l.Debug("Resp from tokendecoder", "empty body")
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	//

	//check quiz
	var quiz repo.Quiz
	if err = conf.mgoCollQuizes.FindId(quizId).Select(bson.M{"questions": 1}).One(&quiz); err != nil {
		if err == mgo.ErrNotFound {
			l.Debug("FindId", "quiz not found")
			return suckhttp.NewResponse(404, "Not found"), nil
		}
		return nil, err
	}
	//

	result.Answers = make(map[string][]string)
	for questionId, answers := range values {
		if _, ok := quiz.Questions[questionId]; !ok {
			l.Warning("Request", "questionId in request doesnt exist in db")
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		result.Answers[questionId] = answers
	}

	result.Datetime = time.Now()

	update := bson.M{"$set": bson.M{"answers": &result.Answers, "entityid": entityId, "quizid": quizId.Hex(), "datetime": result.Datetime, "userid": result.UserId}}
	if _, err = conf.mgoColl.Upsert(bson.M{"quizid": quizId.Hex(), "entityid": entityId, "userid": result.UserId}, update); err != nil {
		return nil, err
	}

	return suckhttp.NewResponse(302, "Found").AddHeader(suckhttp.Location, suckutils.ConcatTwo("/view/", entityId)), nil //?????
}
