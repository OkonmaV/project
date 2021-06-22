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
	"github.com/bradfitz/gomemcache/memcache"
)

type Handler struct {
	conn         *memcache.Client
	template     *template.Template
	decoder      *httpservice.InnerService
	getFolders   *httpservice.InnerService
	getMetausers *httpservice.InnerService
}

type cookieData struct {
	UserId  string `json:"Login"`
	MetaId  string `json:"metaid"`
	Surname string `json:"surname"`
	Name    string `json:"name"`
}

func NewHandler(memcs string, templ *template.Template, auth, tokendecoder, getfolders, getmetausers *httpservice.InnerService) (*Handler, error) {
	conn := memcache.New(memcs)
	err := conn.Ping()
	if err != nil {
		return nil, err
	}
	logger.Info("Checking memcached", memcs)
	return &Handler{conn: conn, template: templ, getFolders: getfolders, getMetausers: getmetausers, decoder: tokendecoder}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	folderId := strings.Trim(r.Uri.Path, "/")
	if folderId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	koki, ok := r.GetCookie("koki")
	if !ok || len(koki) < 5 {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	tokenDecoderReq, err := conf.decoder.CreateRequestFrom(suckhttp.GET, suckutils.Concat("/", koki), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	tokenDecoderReq.AddHeader(suckhttp.Accept, "application/json")
	tokenDecoderResp, err := conf.decoder.Send(tokenDecoderReq)
	if err != nil {
		l.Error("Send", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if i, t := tokenDecoderResp.GetStatus(); i/100 != 2 {
		l.Debug("Resp from tokendecoder", t)
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	if len(tokenDecoderResp.GetBody()) == 0 {
		l.Debug("Resp from tokendecoder", "empty body")
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	userData := &cookieData{}

	if err = json.Unmarshal(tokenDecoderResp.GetBody(), userData); err != nil {
		l.Error("Unmarshal", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	l.Info("UserId", userData.UserId)
	var data templatedata
	data.MetaId = userData.MetaId
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
			l.Error("getmetausersStudent", err)
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
			l.Error("getmetausersNauch", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
	}

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
	MetaId    string
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

	//fmt.Println(string(resp.GetBody()))
	if err := json.Unmarshal(resp.GetBody(), data); err != nil {
		return errors.New(suckutils.ConcatTwo("unmarshal: ", err.Error()))
	}
	return nil
}
