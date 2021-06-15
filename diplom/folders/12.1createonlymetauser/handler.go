package main

import (
	"encoding/json"
	"net/url"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type Handler struct {
	mgoColl *mgo.Collection
	auth    *httpservice.Authorizer
}

type metauser struct {
	MetaId  string `bson:"_id" json:"metaid"`
	Surname string `bson:"surname" json:"surname"`
	Name    string `bson:"name" json:"name"`
	Code    string `bson:"-" json:"regcode"`
}

func NewHandler(mgoColl *mgo.Collection, auth *httpservice.InnerService, tokendecoder *httpservice.InnerService) (*Handler, error) {
	authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	if err != nil {
		return nil, err
	}

	return &Handler{mgoColl: mgoColl, auth: authorizer}, nil
}

func getRandId() string {
	return bson.NewObjectId().Hex()
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || r.GetMethod() != suckhttp.POST {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		l.Error("Parsing r.Body", err)
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}
	//contextFolderId = formValues.Get("contextfid")

	metaSurname := formValues.Get("surname")
	metaName := formValues.Get("name")

	if metaName == "" || metaSurname == "" {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	metaId := getRandId()

	if err = conf.mgoColl.Insert(&metauser{MetaId: metaId, Surname: metaSurname, Name: metaName}); err != nil {
		//TODO: err when founded?
		return nil, err
	}

	resp := suckhttp.NewResponse(200, "OK")
	var body []byte
	var contentType string = "text/plain"
	if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {
		var err error
		body, err = json.Marshal(&metauser{MetaId: metaId, Surname: metaSurname, Name: metaName})
		if err != nil {
			l.Error("Marshalling inserted data", err)
			return resp, nil // ??
		}
		contentType = "application/json"
	} else {
		body = []byte(metaId)
	}
	resp.SetBody(body).AddHeader(suckhttp.Content_Type, contentType)

	return resp, nil
}
