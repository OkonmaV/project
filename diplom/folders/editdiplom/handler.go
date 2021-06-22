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
	getFolders   *httpservice.InnerService
	getMetausers *httpservice.InnerService
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

type templatedata struct {
	Folder    folder
	Student   metauser
	Nauchruk  metauser
	Metausers []metauser
}

func NewHandler(templ *template.Template, auth, tokendecoder, getfolders, getmetausers *httpservice.InnerService) (*Handler, error) {
	authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{template: templ, auth: authorizer, getFolders: getfolders, getMetausers: getmetausers}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.HttpMethod("GET") || !strings.Contains(r.GetHeader(suckhttp.Accept), "text/html") {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	folderId := strings.Trim(r.Uri.Path, "/")

	//	TODO: AUTH
	// k, _, err := conf.auth.GetAccess(r, l, "folders", 1)
	// if err != nil {
	// 	return nil, err
	// }
	// if k {
	// 	return suckhttp.NewResponse(403, "Forbidden"), nil
	// }

	var data templatedata
	// GET FOLDER
	getFoldersReq, err := conf.getFolders.CreateRequestFrom(suckhttp.GET, suckutils.ConcatTwo("/?folderid=", folderId), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	if err = getSomeJsonData(getFoldersReq, conf.getFolders, l, &data.Folder); err != nil {
		l.Error("getfolders", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	//
	for _, metauser := range data.Folder.Metas {
		if metauser.Type == 5 {
			data.Nauchruk.MetaId = metauser.Id
		}
		if metauser.Type == 1 {
			data.Student.MetaId = metauser.Id
		}
	}
	// GET STUDENT

	if data.Student.MetaId != "" {
		getMetausersReq, err := conf.getMetausers.CreateRequestFrom(suckhttp.GET, suckutils.ConcatTwo("/", data.Student.MetaId), r)
		if err != nil {
			l.Error("CreateRequestFrom", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		if err = getSomeJsonData(getMetausersReq, conf.getMetausers, l, &data.Student); err != nil {
			l.Error("getmetausers", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
	} else {
		l.Error("METAUSERS", errors.New(suckutils.ConcatTwo("folder without student, folderid: ", folderId)))
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	//

	// GET NAUCHRUK
	for _, metauser := range data.Folder.Metas {
		if metauser.Type == 5 {
			data.Nauchruk.MetaId = metauser.Id
		}
	}
	if data.Nauchruk.MetaId != "" {
		getMetausersReq, err := conf.getMetausers.CreateRequestFrom(suckhttp.GET, suckutils.ConcatTwo("/", data.Nauchruk.MetaId), r)
		if err != nil {
			l.Error("CreateRequestFrom", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		if err = getSomeJsonData(getMetausersReq, conf.getMetausers, l, &data.Nauchruk); err != nil {
			l.Error("getmetausers", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
	}
	//

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
