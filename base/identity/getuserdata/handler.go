package main

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type Handler struct {
	mgoColl *mgo.Collection
}

func NewHandler(mgoColl *mgo.Collection) (*Handler, error) {
	return &Handler{mgoColl: mgoColl}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	// AUTH?
	if r.GetMethod() != suckhttp.GET {
		l.Debug("Request", "not GET")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	reqData, err := url.ParseQuery(r.Uri.RawQuery)
	if err != nil {
		l.Error("Err parsing query", err)
		return suckhttp.NewResponse(400, "Bad request"), err
	}
	if i, ok := reqData["fields"]; !ok || len(i) == 0 || i[0] == "" {
		l.Debug("Request query", "\"fields\" doesnt exist or empty")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var fields []string

	if len(reqData["fields"]) == 1 {
		fields = strings.Split(reqData["fields"][0], ",")
	} else {
		fields = reqData["fields"]
	}

	userId := strings.Trim(r.Uri.Path, "/")
	if userId == "" {
		l.Debug("Request", "userId (path) not specified")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	selector := make(map[string]interface{}, len(fields))

	for _, fieldName := range fields {
		selector[suckutils.ConcatTwo("data.", fieldName)] = 1
	}

	var mgoRes map[string]interface{}
	if err = conf.mgoColl.FindId(userId).Select(selector).One(&mgoRes); err != nil {
		if err == mgo.ErrNotFound {
			l.Debug("Mongo Find", "not found")
			return suckhttp.NewResponse(404, "Not found"), nil
		}
		return nil, err
	}
	if mgoRes == nil {
		l.Error("Mongo FindId", errors.New("mgoRes is nil"))
	}
	resp := suckhttp.NewResponse(200, "OK")
	var body []byte
	var contentType string
	if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {
		body, err = json.Marshal(mgoRes["data"])
		if err != nil {
			l.Error("Marshalling result", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		contentType = "application/json"
	} else {
		l.Debug("Accept", "not allowed")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	resp.SetBody(body).AddHeader(suckhttp.Content_Type, contentType)

	return resp, nil

}
