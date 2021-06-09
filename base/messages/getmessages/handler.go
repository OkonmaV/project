package main

import (
	"bytes"
	"errors"
	"io/ioutil"
	"strings"
	"text/template"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/suckhttp"
	"go.mongodb.org/mongo-driver/bson"
)

type Handler struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
	template   *template.Template
}

type chat struct {
	Id    string `bson:"_id"`
	Type  int    `bson:"type"`
	Users []user `bson:"users"`
	Name  string `bson:"name,omitempty"`
}
type user struct {
	UserId        string    `bson:"userid"`
	Type          int       `bson:"type"`
	ChatName      string    `bson:"chatname"`
	StartDateTime time.Time `bson:"startdatetime"`
	//EndDateTime   time.Time `bson:"enddatetime"`
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

	rootId := strings.Trim(r.Uri.Path, "/")
	if rootId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// TODO: AUTH
	userId := "testOwner"
	//
	mgoRes := []chat{}

	if err := conf.mgoColl.Find(bson.M{"users.userid:": userId}).All(&mgoRes); err != nil {
		return nil, err
	}

	var body []byte
	var contentType string
	if len(mgoRes) != 0 {
		for i, chatt := range mgoRes {

			if chatt.Type == 1 {
				if len(chatt.Users) != 2 {
					l.Error("Chat", errors.New("chattype unmatches with len(chatusers)"))
					mgoRes[i] = chat{} //????????
					continue           //??????????
				}

				if chatt.Users[0].UserId == userId {
					mgoRes[i].Name = chatt.Users[0].ChatName
				} else {
					mgoRes[i].Name = chatt.Users[1].ChatName
				}
			}
		}
		buf := bytes.NewBuffer(body)
		err := conf.template.Execute(buf, mgoRes)
		if err != nil {
			l.Error("Template execution", err)
			return suckhttp.NewResponse(500, "Internal server error"), err
		}
		body = buf.Bytes()
		contentType = "text/html"
	}

	return suckhttp.NewResponse(200, "OK").SetBody(body).AddHeader(suckhttp.Content_Type, contentType), nil

}
