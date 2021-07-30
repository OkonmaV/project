package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"text/template"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type Handler struct {
	mgoColl  *mgo.Collection
	template *template.Template
}

type metauser struct {
	MetaId  string `bson:"_id" json:"metaid"`
	Surname string `bson:"surname" json:"surname"`
	Name    string `bson:"name" json:"name"`
}

func NewHandler(mgoColl *mgo.Collection, template *template.Template) (*Handler, error) {

	return &Handler{mgoColl: mgoColl, template: template}, nil

}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// AUTH
	if foo, ok := r.GetCookie("koki"); !ok || foo == "" {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	//

	queryValues, err := url.ParseQuery(r.Uri.RawQuery)
	if err != nil {
		l.Error("Err parsing query", err)
		return suckhttp.NewResponse(400, "Bad request"), err
	}

	var query bson.M
	if ids, ok := queryValues["metaid"]; ok && len(ids) != 0 && ids[0] != "" {
		var metauserIds []string
		if len(ids) == 1 {
			metauserIds = strings.Split(ids[0], ",")
		} else {
			metauserIds = ids
		}
		query = bson.M{"_id": bson.M{"$in": metauserIds}}
	}

	mgoRes := []metauser{}

	if err := conf.mgoColl.Find(query).All(&mgoRes); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		return nil, err
	}

	if len(mgoRes) == 0 { // нужно ли???
		l.Error("FindAll", errors.New("empty responce"))
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var body []byte
	var contentType string
	if strings.Contains(r.GetHeader(suckhttp.Accept), "text/html") {

		buf := bytes.NewBuffer(body)

		if err := conf.template.Execute(buf, mgoRes); err != nil {
			l.Error("Template execution", err)
			return suckhttp.NewResponse(500, "Internal server error"), err
		}
		body = buf.Bytes()
		contentType = "text/html"

	} else if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {

		var err error

		body, err = json.Marshal(mgoRes)

		if err != nil {
			l.Error("Marshalling", err)
			return suckhttp.NewResponse(500, "Internal server error"), nil
		}
		contentType = "application/json"

	} else {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	return suckhttp.NewResponse(200, "OK").SetBody(body).AddHeader(suckhttp.Content_Type, contentType), nil

}
