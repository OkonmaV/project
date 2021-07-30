package main

import (
	"net/url"
	"project/diplom/folders/repo"
	"strconv"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type Handler struct {
	mgoColl      *mgo.Collection
	mgoCollUsers *mgo.Collection
	auth         *httpservice.Authorizer
}

func NewHandler(col *mgo.Collection, colUsers *mgo.Collection, auth *httpservice.InnerService, tokendecoder *httpservice.InnerService) (*Handler, error) {
	authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{mgoColl: col, mgoCollUsers: colUsers, auth: authorizer}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.HttpMethod("PATCH") || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || len(r.Body) == 0 {
		l.Debug("Request", "method or content-type not allowed, or empty body")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	folderId, err := bson.NewObjectIdFromHex(strings.Trim(r.Uri.Path, "/"))
	if err != nil {
		l.Debug("Request", "rootid not specified in path")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		l.Debug("Parse body", err.Error())
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	folderNewUserId, err := bson.NewObjectIdFromHex(formValues.Get("userid"))
	if err != nil {
		l.Debug("Request", "userid not specified correctly in body")
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}
	folderNewUserType, err := strconv.Atoi(formValues.Get("fnewmetatype"))
	if err != nil {
		l.Debug("Request", "type not specified correctly in body")
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	// AUTH

	_, _, err = conf.auth.GetAccess(r, l, "folders", 1)
	if err != nil {
		return nil, err
	}
	// if !k {
	// 	return suckhttp.NewResponse(403, "Forbidden"), nil
	// }
	//

	// check user
	if err := conf.mgoCollUsers.Find(bson.M{"_id": folderNewUserId}).Select(bson.M{"_id": 1}).One(nil); err != nil {
		if err == mgo.ErrNotFound {
			l.Debug("Checking new user", "user not found")
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		return nil, err
	}
	// TODO: добавить в запрос к монге тошо юзер-просящий должен находиться в этой папке?
	query := bson.M{"_id": folderId, "deleted": bson.M{"$exists": false}}

	change := mgo.Change{
		Update: bson.M{"$addToSet": bson.M{"users": &repo.User{Id: folderNewUserId, Type: folderNewUserType}}},
		Upsert: false,
		Remove: false,
	}

	// Matched и Updated всегда 1 когда найдено, поэтому неинформативно
	_, err = conf.mgoColl.Find(query).Apply(change, nil)
	if err != nil {
		if err == mgo.ErrNotFound {
			l.Debug("findAndModify", "folder not found")
			return suckhttp.NewResponse(400, "Bad Request"), nil
		}
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil

}
