package main

import (
	"encoding/json"
	"strings"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type GetQuizResults struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
}

type results struct {
	QuizId       string       `bson:"_id" json:"quizid"`
	Usersresults []userresult `bson:"usersresults" json:"usersresults"`
}

type userresult struct {
	UserId      string        `bson:"userid" json:"userid"`
	UserAnswers []useranswers `bson:"useranswers" json:"useranswers"`
	Datetine    time.Time     `bson:"datetime" json:"datetime"`
}

type useranswers struct {
	QuestionId string   `bson:"qid" json:"qid"` //TODO: как брать текст вопросов и ответов?
	AnswersIds []string `bson:"aids,omitempty" json:"aids"`
	Text       string   `bson:"atext,omitempty" json:"atext"`
}

func NewGetQuizResults(mgodb string, mgoAddr string, mgoColl string) (*GetQuizResults, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		logger.Error("Mongo conn", err)
		return nil, err
	}
	logger.Info("Mongo", "Connected!")
	mgoCollection := mgoSession.DB(mgodb).C(mgoColl)

	return &GetQuizResults{mgoSession: mgoSession, mgoColl: mgoCollection}, nil

}

func (conf *GetQuizResults) Close() error {
	conf.mgoSession.Close()
	return nil
}

func (conf *GetQuizResults) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	quizId := strings.Trim(r.Uri.Path, "/")
	if quizId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	userId := strings.TrimSpace(r.Uri.Query().Get("userid"))

	// TODO: AUTH
	var mgoRes results
	var selector bson.M
	query := bson.M{"_id": quizId}

	if userId != "" {
		selector = bson.M{"usersresults.$": 1}
		query["userresults.userid"] = userId
	}
	if err := conf.mgoColl.Find(bson.M{"_id": quizId}).Select(selector).One(&mgoRes); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	var body []byte
	var contentType string

	if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {
		var err error
		body, err = json.Marshal(mgoRes)
		if err != nil {
			l.Error("Marshalling mongo responce", err)
			return suckhttp.NewResponse(500, "Internal server error"), nil
		}
		contentType = "application/json"
	}

	return suckhttp.NewResponse(200, "OK").SetBody(body).AddHeader(suckhttp.Content_Type, contentType), nil
}
