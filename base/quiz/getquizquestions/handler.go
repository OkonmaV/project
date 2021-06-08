package main

import (
	"bytes"
	"html/template"
	"io/ioutil"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/suckhttp"
	"go.mongodb.org/mongo-driver/bson"
)

type GetQuizQuestions struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
	template   *template.Template
}
type quiz struct {
	Id        string              `bson:"_id" json:"quizid"`
	Name      string              `bson:"name" json:"quizname"`
	Questions map[string]question `bson:"questions" json:"questions"`
	CreatorId string              `bson:"creatorid" json:"creatorid"`
}

type question struct {
	Type     int               `bson:"question_type" json:"question_type"`
	Position int               `bson:"question_position" json:"question_position"`
	Text     string            `bson:"question_text" json:"question_text"`
	Answers  map[string]string `bson:"answers" json:"answers"`
}

func NewGetQuizQuestions(mgodb string, mgoAddr string, mgoColl string) (*GetQuizQuestions, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		logger.Error("Mongo conn", err)
		return nil, err
	}
	logger.Info("Mongo", "Connected!")
	mgoCollection := mgoSession.DB(mgodb).C(mgoColl)

	templData, err := ioutil.ReadFile("index.html")
	if err != nil {
		return nil, err
	}

	templ, err := template.New("index").Parse(string(templData))
	if err != nil {
		return nil, err
	}

	return &GetQuizQuestions{mgoSession: mgoSession, mgoColl: mgoCollection, template: templ}, nil

}

func (conf *GetQuizQuestions) Close() error {
	conf.mgoSession.Close()
	return nil
}

func (conf *GetQuizQuestions) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	quizId := strings.Trim(r.Uri.Path, "/")
	if quizId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// TODO: AUTH
	var mgoRes quiz

	if err := conf.mgoColl.Find(bson.M{"_id": quizId, "deleted": bson.M{"$exists": false}}).Select(bson.M{"questions": 1}).One(&mgoRes); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	var body []byte
	var contentType string

	if len(mgoRes.Questions) != 0 {
		buf := bytes.NewBuffer(body)
		err := conf.template.Execute(buf, mgoRes.Questions)
		if err != nil {
			l.Error("Template execution", err)
			return suckhttp.NewResponse(500, "Internal server error"), err
		}
		body = buf.Bytes()
		contentType = "text/html"
	}

	return suckhttp.NewResponse(200, "OK").SetBody(body).AddHeader(suckhttp.Content_Type, contentType), nil
}
