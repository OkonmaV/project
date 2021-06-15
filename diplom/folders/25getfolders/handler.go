package main

import (
	"bytes"
	"errors"
	"strings"
	"text/template"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/suckhttp"
	"go.mongodb.org/mongo-driver/bson"
)

type Handler struct {
	mgoColl  *mgo.Collection
	template *template.Template
}

type folder struct {
	Id string `bson:"_id"`
	//RootsId []string `bson:"rootsid"`
	Name string `bson:"name"`
	//Metas   []interface{} `bson:"metas"`
}

func NewHandler(mgoColl *mgo.Collection, template *template.Template) (*Handler, error) {

	return &Handler{mgoColl: mgoColl, template: template}, nil

}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	rootId := strings.Trim(r.Uri.Path, "/")
	if rootId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// AUTH
	if foo, ok := r.GetCookie("koki"); !ok || foo == "" {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	// TODO: get metaid
	mgoRes := []folder{}

	if err := conf.mgoColl.Find(bson.M{"rootsid": rootId}).Select(bson.M{"rootsid": 0, "metas": 0}).All(&mgoRes); err != nil {
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
