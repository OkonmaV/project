package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"text/template"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type Handler struct {
	mgoColl      *mgo.Collection
	template     *template.Template
	getMetausers *httpservice.InnerService
}

type folder struct {
	Id       string   `bson:"_id" json:"_id"`
	Name     string   `bson:"name" json:"name"`
	Metas    []meta   `bson:"metas" json:"metas"`
	Student  metauser `bson:"-" json:"student"`
	Nauchruk metauser `bson:"-" json:"nauchruk"`
	// RootsId    []string `bson:"rootsid" json:"-"`
	// Type       int      `bson:"type" json:"type"`
	// Speciality string   `bson:"speciality" json:"speciality"`
}

type metauser struct {
	MetaId  string `bson:"_id" json:"metaid"`
	Surname string `bson:"surname" json:"surname"`
	Name    string `bson:"name" json:"name"`
}

type meta struct {
	Type int    `bson:"metatype" json:"metatype"`
	Id   string `bson:"metaid" json:"metaid"`
}

func NewHandler(mgoColl *mgo.Collection, template *template.Template, getmetausers *httpservice.InnerService) (*Handler, error) {

	return &Handler{mgoColl: mgoColl, template: template, getMetausers: getmetausers}, nil

}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// AUTH
	if foo, ok := r.GetCookie("koki"); !ok || foo == "" {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	// TODO: get metaid

	var body []byte
	var contentType string

	if strings.Contains(r.GetHeader(suckhttp.Accept), "text/html") {

		rootId := strings.Trim(r.Uri.Path, "/")
		if rootId == "" {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}

		mgoRes := []folder{}
		if err := conf.mgoColl.Find(bson.M{"rootsid": rootId, "type": 5}).Select(bson.M{"rootsid": 0, "type": 0}).All(&mgoRes); err != nil {
			return nil, err
		}

		for i, fldr := range mgoRes {

		}

		buf := bytes.NewBuffer(body)
		err := conf.template.Execute(buf, mgoRes)
		if err != nil {
			l.Error("Template execution", err)
			return suckhttp.NewResponse(500, "Internal server error"), err
		}
		body = buf.Bytes()
		contentType = "text/html"

	} else if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {

		mgoRes := folder{}
		folderId := r.Uri.Query().Get("folderid")
		if folderId == "" {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}

		if err := conf.mgoColl.FindId(folderId).Select(bson.M{"rootsid": 0, "_id": 0}).One(&mgoRes); err != nil {
			if err == mgo.ErrNotFound {
				l.Error("FindId", err)
				return suckhttp.NewResponse(400, "Bad request"), nil
			}
			return nil, err
		}

		var err error
		if body, err = json.Marshal(mgoRes); err != nil {
			l.Error("Marshal", err)
			return suckhttp.NewResponse(500, "Internal server error"), nil
		}
		contentType = "application/json"
	} else {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	return suckhttp.NewResponse(200, "OK").SetBody(body).AddHeader(suckhttp.Content_Type, contentType), nil

}

func getSomeJsonData(req *suckhttp.Request, conn *httpservice.InnerService, l *logger.Logger, data interface{}) error {

	req.AddHeader(suckhttp.Accept, "application/json")
	resp, err := conn.Send(req)
	if err != nil {
		return errors.New(suckutils.ConcatTwo("send: ", err.Error()))
	}

	if i, t := resp.GetStatus(); i/100 != 2 {
		return errors.New(suckutils.ConcatTwo("status: ", t))
	}
	if len(resp.GetBody()) == 0 {
		return errors.New("body: is empty")
	}

	if err := json.Unmarshal(resp.GetBody(), data); err != nil {
		return errors.New(suckutils.ConcatTwo("unmarshal: ", err.Error()))
	}
	return nil
}
