package main

import (
	"encoding/json"
	"net/url"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/suckhttp"
	"go.mongodb.org/mongo-driver/bson"
)

type GetUserData struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
}

func NewGetUserData(mgodb string, mgoAddr string, mgoColl string) (*GetUserData, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		return nil, err
	}
	logger.Info("Mongo", "Connected!")

	return &GetUserData{mgoSession: mgoSession, mgoColl: mgoSession.DB(mgodb).C(mgoColl)}, nil
}

func (c *GetUserData) Close() error {
	c.mgoSession.Close()
	return nil
}

func (conf *GetUserData) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	// AUTH?
	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	reqData, err := url.ParseQuery(r.Uri.RawQuery)
	if err != nil {
		l.Error("Err parsing query", err)
		return suckhttp.NewResponse(400, "Bad request"), err
	}
	userId := r.Uri.Path
	if userId == "" {
		l.Debug("Request path", "\"_id\" dosnt exist or empty")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	if i, ok := reqData["fields"]; !ok || len(i) == 0 || i[0] == "" {
		l.Debug("Request query", "\"fields\" dosnt exist or empty")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var mgoRes map[string]interface{}
	if err = conf.mgoColl.FindId(userId).Select(&bson.M{"data": reqData["fields"]}).One(&mgoRes); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		return nil, err
	}

	resp := suckhttp.NewResponse(200, "OK")
	var body []byte
	var contentType string
	if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {
		body, err = json.Marshal(mgoRes)
		if err != nil {
			l.Error("Marshalling result", err)
			return nil, nil
		}
		contentType = "application/json"
	}

	resp.SetBody(body)
	resp.AddHeader(suckhttp.Content_Type, contentType)

	return resp, nil

}
