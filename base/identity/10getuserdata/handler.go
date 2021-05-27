package main

import (
	"encoding/json"
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

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/json") {
		l.Debug("Content-type", "Wrong content-type at POST")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	if len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}
	var reqData map[string][]string
	err := json.Unmarshal(r.Body, &reqData)
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), err
	}

	if _, ok := reqData["fields"]; !ok {
		l.Debug("Request json", "\"fields\" field is nil")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	if _, ok := reqData["_id"]; !ok {
		l.Debug("Request json", "\"_id\" field is nil")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var mgoRes map[string]interface{}
	err = conf.mgoColl.FindId(reqData["_id"]).One(&mgoRes)
	if err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(400, "Bad request"), err
		}
		return nil, err
	}

	result := make(map[string]interface{})
	var ok bool
	for _, field := range reqData["fields"] {
		result[field], ok = mgoRes[field]
		if !ok {
			return suckhttp.NewResponse(400, "Bad request"), nil // ??
		}
	}

	resp := suckhttp.NewResponse(200, "OK")
	var body []byte
	var contentType string
	if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {
		body, err = json.Marshal(result)
		if err != nil {
			return nil, err
		}
		contentType = "application/json"
	}

	resp.SetBody(body)
	resp.AddHeader(suckhttp.Content_Type, contentType)

	return resp, nil

}
