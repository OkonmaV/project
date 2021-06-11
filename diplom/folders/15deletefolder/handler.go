package main

import (
	"errors"
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
type authreqdata struct {
	MetaId string
}

func NewHandler(col *mgo.Collection, auth *httpservice.InnerService, tokendecoder *httpservice.InnerService) (*Handler, error) {
	authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{mgoColl: col, auth: authorizer}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.DELETE {
		return suckhttp.NewResponse(405, "Method not allowed"), nil
	}

	fid := r.Uri.Path
	fid = strings.Trim(fid, "/")
	if fid == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	userData := &authreqdata{}
	k, err := conf.auth.GetAccessWithData(r, l, "folders", 1, userData)
	if err != nil {
		return nil, err
	}
	if !k {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	if userData.MetaId == "" {
		l.Error("GetAccessWithData", errors.New("no metaid in resp"))
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	query := &bson.M{"_id": fid, "deleted": bson.M{"$exists": false}}

	change := mgo.Change{
		Update:    bson.M{"$set": bson.M{"deleted.by": userData.MetaId}},
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
