package main

import (
	"encoding/json"
	"net/url"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/suckhttp"
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
 // check GET
	reqData, err := url.ParseQuery(r.Uri.RawQuery)
	if err != nil {
		l.Error("Err parsing query", err)
		return suckhttp.NewResponse(400, "Bad request"), err
	}

	if i, ok := reqData["fields"]; !ok || len(i) != 0 {
		l.Debug("Request query", "\"fields\" dosnt exist or empty")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	if i, ok := reqData["id"]; !ok || len(i) != 0 {
		l.Debug("Request query", "\"_id\" dosnt exist or empty")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var mgoRes map[string]interface{} // через селектор пихаем поля
	if err = conf.mgoColl.FindId(reqData["_id"][0]).Select().One(&mgoRes); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		return nil, err
	}
 	var smth// я хз как вытащить данные, при учете что они лежат во вложенном в документ data. впиливать структуру в mgoRes?????
	result := make(map[string]interface{}, len(reqData["fields"]))
	var ok bool
	for _, field := range reqData["fields"] {
		result[field], ok = mgoRes[field]
		if !ok {
			return suckhttp.NewResponse(400, "Bad request"), nil // ???????
		}
	}

	resp := suckhttp.NewResponse(200, "OK")
	var body []byte
	var contentType string
	if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {
		body, err = json.Marshal(result)
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
