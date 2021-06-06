package main

import (
	"encoding/json"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/suckhttp"
	"go.mongodb.org/mongo-driver/bson"
)

type GetQuizQuestions struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
}
type quiz struct {
	Id        string     `bson:"_id" json:"quizid"`
	Name      string     `bson:"name" json:"quizname"`
	Questions []question `bson:"questions" json:"questions"`
	CreatorId string     `bson:"creatorid" json:"creatorid"`
}

type question struct {
	Id       string   `bson:"qid" json:"qid"`
	Type     int      `bson:"qtype" json:"qtype"`
	Position int      `bson:"qposition" json:"position"`
	Text     string   `bson:"qtext" json:"qtext"`
	Answers  []answer `bson:"answers" json:"answers"`
}

type answer struct {
	Id   string `bson:"aid" json:"aid"`
	Text string `bson:"atext" json:"atext,omitempty"`
}

func NewGetQuizQuestions(mgodb string, mgoAddr string, mgoColl string) (*GetQuizQuestions, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		logger.Error("Mongo conn", err)
		return nil, err
	}
	logger.Info("Mongo", "Connected!")
	mgoCollection := mgoSession.DB(mgodb).C(mgoColl)

	return &GetQuizQuestions{mgoSession: mgoSession, mgoColl: mgoCollection}, nil

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
