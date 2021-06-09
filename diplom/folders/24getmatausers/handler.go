package main

import (
	"bytes"
	"errors"
	"io/ioutil"
	"text/template"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/suckhttp"
	"go.mongodb.org/mongo-driver/bson"
)

type Handler struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
	template   *template.Template
}

type metauser struct {
	MetaId  string `bson:"_id" json:"metaid"`
	Surname string `bson:"surname" json:"surname"`
	Name    string `bson:"name" json:"name"`
}

func NewHandler(mgodb string, mgoAddr string, mgoColl string) (*Handler, error) {

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

	return &Handler{mgoSession: mgoSession, mgoColl: mgoCollection, template: templ}, nil

}

func (conf *Handler) Close() error {
	conf.mgoSession.Close()
	return nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// TODO: AUTH
	mgoRes := []metauser{}

	if err := conf.mgoColl.Find(bson.M{}).All(&mgoRes); err != nil {
		return nil, err
	}

	if len(mgoRes) == 0 { // нужно ли???
		l.Error("FindAll", errors.New("empty responce"))
		return suckhttp.NewResponse(500, "Internal server error"), nil
	}

	var body []byte
	buf := bytes.NewBuffer(body)
	err := conf.template.Execute(buf, mgoRes)
	if err != nil {
		l.Error("Template execution", err)
		return suckhttp.NewResponse(500, "Internal server error"), err
	}
	body = buf.Bytes()
	contentType := "text/html"

	return suckhttp.NewResponse(200, "OK").SetBody(body).AddHeader(suckhttp.Content_Type, contentType), nil

}
