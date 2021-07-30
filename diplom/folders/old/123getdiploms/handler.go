package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
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
	auth         *httpservice.Authorizer
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

type cookieData struct {
	MetaId string `json:"metaid"`
	Role   int    `json:"role"`
}

func NewHandler(mgoColl *mgo.Collection, template *template.Template, auth, tokendecoder, getmetausers *httpservice.InnerService) (*Handler, error) {

	authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{mgoColl: mgoColl, auth: authorizer, template: template, getMetausers: getmetausers}, nil

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

	rootId := strings.Trim(r.Uri.Path, "/")
	if rootId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var cookieClaims cookieData
	_, err := conf.auth.GetAccessWithData(r, l, "folders", 1, &cookieClaims)
	if err != nil {
		return nil, err
	}
	// if !k {
	// 	return suckhttp.NewResponse(403, "Forbidden"), nil
	// }

	mgoRes := []*folder{}
	if err := conf.mgoColl.Find(bson.M{"rootsid": rootId, "type": 5}).Select(bson.M{"rootsid": 0, "type": 0}).All(&mgoRes); err != nil {
		return nil, err
	}

	var metaids []string
	metausersIndex := make(map[string]*metauser)

	for _, fldr := range mgoRes {
		for _, metausr := range fldr.Metas {
			if metausr.Type == 5 {
				//fldr.Nauchruk = metauser{MetaId: metausr.Id}
				metaids = append(metaids, metausr.Id)
				metausersIndex[metausr.Id] = &fldr.Nauchruk
			}
			if metausr.Type == 1 {
				//fldr.Student = metauser{MetaId: metausr.Id}
				metaids = append(metaids, metausr.Id)
				metausersIndex[metausr.Id] = &fldr.Student
			}
		}
	}
	// GET METAUSERS
	getMetausersReq, err := conf.getMetausers.CreateRequestFrom(suckhttp.GET, suckutils.ConcatTwo("/?metaid=", strings.Join(metaids, ",")), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal server error"), nil
	}

	var mgoResMetausers []metauser
	if err = getSomeJsonData(getMetausersReq, conf.getMetausers, &mgoResMetausers); err != nil {
		l.Error("getMetausers", err)
		return suckhttp.NewResponse(500, "Internal server error"), nil
	}
	//

	if c := len(metaids) - len(mgoResMetausers); c != 0 {
		l.Error("METAUSERS", errors.New(suckutils.Concat("cant find ", strconv.Itoa(c), " metausers in mongo")))
		// NO RETURN
	}

	for _, metausr := range mgoResMetausers {
		if metausersIndex[metausr.MetaId] != nil {
			*metausersIndex[metausr.MetaId] = metausr
		}
	}

	if strings.Contains(r.GetHeader(suckhttp.Accept), "text/html") {

		buf := bytes.NewBuffer(body)
		if err := conf.template.Execute(buf, struct {
			User    *cookieData
			Folders []*folder
		}{
			User:    &cookieClaims,
			Folders: mgoRes}); err != nil {
			l.Error("Template execution", err)
			return suckhttp.NewResponse(500, "Internal server error"), err
		}
		body = buf.Bytes()
		contentType = "text/html"

	} else if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {

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

func getSomeJsonData(req *suckhttp.Request, conn *httpservice.InnerService, data interface{}) error {

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
