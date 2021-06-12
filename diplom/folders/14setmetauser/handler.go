package main

import (
	"errors"
	"strconv"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type Handler struct {
	mgoColl          *mgo.Collection
	mgoCollMetausers *mgo.Collection
	auth             *httpservice.Authorizer
}

type meta struct {
	Type int    `bson:"metatype"`
	Id   string `bson:"metaid"`
}

type authreqdata struct {
	Metaid string `json:"metaid"`
}

func NewHandler(col *mgo.Collection, colMeta *mgo.Collection, auth *httpservice.InnerService, tokendecoder *httpservice.InnerService) (*Handler, error) {
	authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{mgoColl: col, mgoCollMetausers: colMeta, auth: authorizer}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.HttpMethod("PATCH") {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	folderid := r.Uri.Path
	folderid = strings.Trim(folderid, "/")
	fnewmeta := strings.TrimSpace(r.Uri.Query().Get("fnewmetaid"))
	if folderid == "" || fnewmeta == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	fnewmetatype, err := strconv.Atoi(r.Uri.Query().Get("fnewmetatype"))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	k, _, err := conf.auth.GetAccess(r, l, "folders", 1)
	if err != nil {
		return nil, err
	}
	if !k {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	if err := conf.mgoCollMetausers.Find(bson.M{"_id": fnewmeta}).Select(bson.M{"_id": 1}).One(nil); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	query := bson.M{"_id": folderid, "deleted": bson.M{"$exists": false}}

	change := bson.M{"$addToSet": bson.M{"metas": &meta{Id: folderid, Type: fnewmetatype}}}

	changeInfo, err := conf.mgoColl.UpdateAll(query, change)
	if err != nil {
		return nil, err
	}
	if changeInfo.Matched == 0 {
		l.Error("AUTH", errors.New("approved folderid doesn't match"))
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	switch changeInfo.Updated {
	case 1:
		return suckhttp.NewResponse(201, "Created"), nil
	case 0:
		return suckhttp.NewResponse(200, "OK"), nil
	default:
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
}
