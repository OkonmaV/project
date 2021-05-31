package main

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type SetMetaUser struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
}

type meta struct {
	Type int    `bson:"metatype"`
	Id   string `bson:"metaid"`
}

func NewSetMetaUser(mgodb string, mgoAddr string, mgoColl string) (*SetMetaUser, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		logger.Error("Mongo conn", err)
		return nil, err
	}

	mgoCollection := mgoSession.DB(mgodb).C(mgoColl)

	return &SetMetaUser{mgoSession: mgoSession, mgoColl: mgoCollection}, nil

}

func (conf *SetMetaUser) Close() error {
	conf.mgoSession.Close()
	return nil
}

func (conf *SetMetaUser) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {
	// Используем метод PATCH со всеми вытекающими. FolderId берем из Uri.Path, metauserid из QueryString
	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		l.Error("Parsing r.Body", err)
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	folderid := formValues.Get("folderid")
	fnewmeta := formValues.Get("fnewmetaid")
	fnewmetatype, err := strconv.Atoi(formValues.Get("fnewmetatype"))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}
	if folderid == "" || fnewmeta == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// TODO: AUTH

	query := &bson.M{"_id": folderid, "deleted": bson.M{"$exists": false}}

	change := mgo.Change{
		Update:    bson.M{"$addToSet": bson.M{"metas": &meta{Type: fnewmetatype, Id: folderid}}},
		Upsert:    false,
		ReturnNew: true,
		Remove:    false,
	}

	// Добавь проверку на измененность документа. В случае, если пользователь уже был добавлен, возвращаем 200 OK, а если добавился то 201 Created
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
		return nil, nil
	}
}
