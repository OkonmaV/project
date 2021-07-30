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
	mgoColl *mgo.Collection
	auth    *httpservice.Authorizer
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
		l.Debug("Request", "method or content-type not allowed, or empty body")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	folderId, err := bson.NewObjectIdFromHex(strings.Trim(r.Uri.Path, "/"))
	if err != nil {
		l.Debug("Request", "folderid not correctly specified in path")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		l.Debug("Parse body", err.Error())
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	folderNewRootId, err := bson.NewObjectIdFromHex(formValues.Get("folderid"))
	if err != nil {
		l.Debug("Request", "new rootid not correctly specified in body")
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

	if err := conf.mgoColl.Find(bson.M{"_id": folderNewRootId}).Select(bson.M{"_id": 1}).One(nil); err != nil {
		if err == mgo.ErrNotFound {
			l.Debug("Check new root", "folder not found")
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		return nil, err
	}

	query := bson.M{"_id": folderId, "deleted": bson.M{"$exists": false}}

	change := bson.M{"$addToSet": bson.M{"rootsid": folderNewRootId}}

	// не нравится updateAll
	changeInfo, err := conf.mgoColl.UpdateAll(query, change)
	if err != nil {
		return nil, err
	}
	if changeInfo.Matched == 0 {
		l.Debug("Update", "folder not found")
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
