package main

import (
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type Handler struct {
	mgoColl        *mgo.Collection
	mgoCollDeleted *mgo.Collection
	auth           *httpservice.Authorizer
}
type authreqdata struct {
	MetaId string
}

func NewHandler(col *mgo.Collection, colDel *mgo.Collection, auth *httpservice.InnerService, tokendecoder *httpservice.InnerService) (*Handler, error) {
	authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{mgoColl: col, mgoCollDeleted: colDel, auth: authorizer}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.DELETE {
		l.Debug("Request", "method not allowed")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	folderId, err := bson.NewObjectIdFromHex(strings.Trim(r.Uri.Path, "/"))
	if err != nil {
		l.Debug("Request", "folderid not correctly specified in path")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// AUTH
	_, _, err = conf.auth.GetAccess(r, l, "folders", 1)
	if err != nil {
		return nil, err
	}
	// if !k {
	// 	return suckhttp.NewResponse(403, "Forbidden"), nil
	// }

	query := &bson.M{"_id": folderId, "deleted": bson.M{"$exists": false}}

	change := mgo.Change{
		Update:    bson.M{"$set": bson.M{"deleted": 1}},
		Upsert:    false,
		ReturnNew: true,
		Remove:    false,
	}

	if _, err := conf.mgoColl.Find(query).Apply(change, nil); err != nil {
		if err == mgo.ErrNotFound {
			l.Debug("FindAndModify", "folder not found")
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
