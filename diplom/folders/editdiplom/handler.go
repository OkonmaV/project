package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type Handler struct {
	template     *template.Template
	auth         *httpservice.Authorizer
	getMetausers *httpservice.InnerService
	viewDiplom   *httpservice.InnerService
}

type folder struct {
	Id         string   `bson:"_id" json:"-"`
	RootsId    []string `bson:"rootsid" json:"-"`
	Name       string   `bson:"name" json:"name"`
	Metas      []meta   `bson:"metas" json:"metas"`
	Type       int      `bson:"type" json:"type"`
	Speciality string   `bson:"speciality" json:"speciality"`
}

type meta struct {
	Type int    `bson:"metatype" json:"metatype"`
	Id   string `bson:"metaid" json:"metaid"`
}

type metauser struct {
	MetaId  string `bson:"_id" json:"metaid"`
	Surname string `bson:"surname" json:"surname"`
	Name    string `bson:"name" json:"name"`
}

type cookieData struct {
	MetaId string `json:"metaid"`
	Role   int    `json:"role"`
}

type templatedata struct {
	Folder               folder
	Student              metauser
	Nauchruk             metauser
	Metausers            []metauser
	BecomeNauchrukMetaId string
	CanChangeTheme       bool
}

func NewHandler(templ *template.Template, auth, tokendecoder, getmetausers, viewdiplom *httpservice.InnerService) (*Handler, error) {
	authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{template: templ, auth: authorizer, getMetausers: getmetausers, viewDiplom: viewdiplom}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.HttpMethod("GET") || !strings.Contains(r.GetHeader(suckhttp.Accept), "text/html") {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	folderId := strings.Trim(r.Uri.Path, "/") //diplomId
	if folderId == "" {
		l.Debug("FolderId", "is nil")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var data templatedata
	//	TODO: AUTH
	var cookieClaims cookieData
	k, err := conf.auth.GetAccessWithData(r, l, "folders", 1, &cookieClaims)
	if err != nil {
		return nil, err
	}
	if !k {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	if cookieClaims.Role > 1 { ///////////////////////////////// setmetauser needs auth with key "folders"
		data.BecomeNauchrukMetaId = cookieClaims.MetaId
	}

	// GET DIPLOM
	viewDiplomReq, err := conf.viewDiplom.CreateRequestFrom(suckhttp.GET, suckutils.ConcatTwo("/", folderId), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	if err = getSomeJsonData(viewDiplomReq, conf.viewDiplom, l, &data); err != nil {
		l.Error("viewDiplom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	//

	if cookieClaims.MetaId == data.Student.MetaId || cookieClaims.MetaId == data.Nauchruk.MetaId {
		data.CanChangeTheme = true
	}

	// GET METAUSERS
	getMetausersReq, err := conf.getMetausers.CreateRequestFrom(suckhttp.GET, "/", r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	if err = getSomeJsonData(getMetausersReq, conf.getMetausers, l, &data.Metausers); err != nil {
		l.Error("getmetausers", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	//

	var body []byte
	buf := bytes.NewBuffer(body)

	if err = conf.template.Execute(buf, data); err != nil {
		l.Error("Template execution", err)
		return suckhttp.NewResponse(500, "Internal server error"), err
	}
	body = buf.Bytes()
	contentType := "text/html"

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

	//fmt.Println(resp.GetBody())
	if err := json.Unmarshal(resp.GetBody(), data); err != nil {
		return errors.New(suckutils.ConcatTwo("unmarshal: ", err.Error()))
	}
	return nil
}
