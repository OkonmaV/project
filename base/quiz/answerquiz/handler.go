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
)

type Handler struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
	auth       *httpservice.Authorizer
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
	QuizId       string       `bson:"_id" json:"quizid"`
	EntityId     string       `bson:"entityid" json:"entityid"`
	Usersresults []userresult `bson:"usersresults" json:"usersresults"`
}

type userresult struct {
	UserId   string              `bson:"userid" json:"userid"`
	Answers  map[string][]string `bson:"answers" json:"answers"`
	Datetime time.Time           `bson:"datetime" json:"datetime"`
}

//
func NewHandler(col *mgo.Collection, authGet *httpservice.InnerService, tokendecoder *httpservice.InnerService) (*Handler, error) {
	authorizerGet, err := httpservice.NewAuthorizer(thisServiceName, authGet, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{mgoColl: col, auth: authorizerGet}, nil
}

func (conf *Handler) Close() error {
	conf.mgoSession.Close()
	return nil
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

	k, userId, err := conf.auth.GetAccess(r, l, quizId.Hex(), 1)
	if err != nil {
		return nil, err
	}
	if !k {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	var mgoRes quiz
	if err = conf.mgoColl.FindId(quizId).Select(bson.M{"questions": 1}).One(&mgoRes); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), err
		}
		return nil, err
	}

	var result userresult
	result.Answers = make(map[string][]string)

	for questionId, answers := range values {
		if _, ok := mgoRes.Questions[questionId]; !ok {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		result.Answers[questionId] = answers

	}

	result.UserId = userId
	result.Datetime = time.Now()

	update := bson.M{"$set": bson.M{"userresults": &result}}
	if _, err = conf.mgoColl.UpsertId(quizId, update); err != nil {
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
