package main

import (
	"net/url"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/suckhttp"
	"github.com/rs/xid"
)

type CreateQuiz struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
}
type quiz struct {
	Id        string     `bson:"_id"`
	Name      string     `bson:"name"`
	Questions []question `bson:"questions"`
	CreatorId string     `bson:"creatorid"`
}

type question struct {
	Id       string   `bson:"question_id"`
	Type     int      `bson:"question_type"`
	Position int      `bson:"question_position"`
	Text     string   `bson:"question_text"`
	Answers  []answer `bson:"answers"`
}

type answer struct {
	Id   string `bson:"answer_id"`
	Text string `bson:"answer_text"`
	//Scores int
}

func NewCreateQuiz(mgodb string, mgoAddr string, mgoColl string) (*CreateQuiz, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		logger.Error("Mongo conn", err)
		return nil, err
	}
	logger.Info("Mongo", "Connected!")
	mgoCollection := mgoSession.DB(mgodb).C(mgoColl)

	return &CreateQuiz{mgoSession: mgoSession, mgoColl: mgoCollection}, nil

}

func (conf *CreateQuiz) Close() error {
	conf.mgoSession.Close()
	return nil
}

func (conf *CreateQuiz) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.PUT || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	quizName := formValues.Get("quizname")
	if quizName == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// TODO: AUTH
	creatorid := "creator"
	//

	quizId := xid.New()
	//нет проверки по имени

	if err = conf.mgoColl.Insert(&quiz{Id: quizId.String(), Name: quizName, CreatorId: creatorid}); err != nil {
		return nil, err
	}

	resp := suckhttp.NewResponse(201, "Created")
	if strings.Contains(r.GetHeader(suckhttp.Accept), "text/plain") {
		resp.SetBody(quizId.Bytes()).AddHeader(suckhttp.Content_Type, "text/plain")
	}
	return resp, nil
}
