package main

import (
	"net/url"
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

//quiz
type quiz struct {
	Questions map[string]question `bson:"questions"`
}

type question struct {
	Type     int               `bson:"question_type"`
	Position int               `bson:"question_position"`
	Text     string            `bson:"question_text"`
	Answers  map[string]answer `bson:"question_answers"`
}

type answer struct {
	Text string `bson:"answer_text"`
	//Scores int
}

//
//results
type results struct {
	QuizId       string       `bson:"_id" json:"quizid"`
	Usersresults []userresult `bson:"usersresults" json:"usersresults"`
}

type userresult struct {
	UserId   string              `bson:"userid" json:"userid"`
	Answers  map[string][]string `bson:"answers" json:"answers"`
	Datetime time.Time           `bson:"datetime" json:"datetime"`
}

//
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
	var ds suckhttp.HttpMethod
	ds = "asd"
	if r.GetMethod() != suckhttp.POST || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	quizId := strings.Trim(r.Uri.Path, "/")
	if quizId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	values, err := url.ParseQuery(string(r.Body))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// TODO: AUTH
	userId := "testuserid"
	//

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

	var body []byte
	var contentType string

	return suckhttp.NewResponse(200, "OK").SetBody(body).AddHeader(suckhttp.Content_Type, contentType), nil
}
