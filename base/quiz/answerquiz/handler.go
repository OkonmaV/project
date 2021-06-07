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

type AnswerQuiz struct {
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
	QuestionId string   `bson:"question_id" json:"question_id"`
	Type       int      `bson:"question_type" json:"question_type"`
	AnswersIds []string `bson:"answer_ids,omitempty" json:"answer_ids,omitempty"`
	Text       string   `bson:"answer_text,omitempty" json:"answer_text,omitempty"`
}

func NewAnswerQuiz(mgodb string, mgoAddr string, mgoColl string) (*AnswerQuiz, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		logger.Error("Mongo conn", err)
		return nil, err
	}
	logger.Info("Mongo", "Connected!")
	mgoCollection := mgoSession.DB(mgodb).C(mgoColl)

	return &AnswerQuiz{mgoSession: mgoSession, mgoColl: mgoCollection}, nil

}

func (conf *AnswerQuiz) Close() error {
	conf.mgoSession.Close()
	return nil
}

func (conf *AnswerQuiz) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/json") || len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	quizId := strings.Trim(r.Uri.Path, "/")
	if quizId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	questId := strings.TrimSpace(r.Uri.Query().Get("questid"))

	var userAnswers map[string]interface{}
	err := json.Unmarshal(r.Body, &userAnswers)
	if err != nil {
		l.Error("Marshalling r.Body", err)
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// TODO: AUTH
	userId := "testuserid"
	//

	userAnswers["userid"] = userId
	userAnswers["datetime"] = time.Now()

	update := bson.M{"$setOnInsert": bson.M{"_id": questId}, "$set": bson.M{"userresults": &userAnswers}}
	if _, err = conf.mgoColl.UpsertId(questId, update); err != nil {
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
