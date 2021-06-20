package main

import (
	"net/url"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type Handler struct {
	auth    *httpservice.Authorizer
	mgoColl *mgo.Collection
}

func NewHandler(col *mgo.Collection, auth *httpservice.InnerService, tokendecoder *httpservice.InnerService) (*Handler, error) {
	authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{mgoColl: col, auth: authorizer}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.HttpMethod("PATCH") || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	fid := strings.Trim(r.Uri.Path, "/")

	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		l.Debug("ParseQuery in body", err.Error())
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	fnewname := formValues.Get("fnewname")
	if fid == "" || fnewname == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	k, _, err := conf.auth.GetAccess(r, l, "folders", 1)
	if err != nil {
		return nil, err
	}
	if !k {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	query := &bson.M{"_id": fid, "deleted": bson.M{"$exists": false}}

	change := mgo.Change{
		Update:    &bson.M{"$set": bson.M{"name": fnewname}},
		Upsert:    false,
		ReturnNew: true,
		Remove:    false,
	}

	if _, err := conf.mgoColl.Find(query).Apply(change, nil); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
