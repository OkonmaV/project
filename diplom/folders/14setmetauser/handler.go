package main

import (
	"errors"
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

	if r.GetMethod() != suckhttp.HttpMethod("PATCH") {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	folderid := r.Uri.Path
	fnewmeta := strings.TrimSpace(r.Uri.Query().Get("fnewmetaid"))
	if folderid == "" || fnewmeta == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	fnewmetatype, err := strconv.Atoi(r.Uri.Query().Get("fnewmetatype"))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	// TODO: AUTH

	query := &bson.M{"_id": folderid, "deleted": bson.M{"$exists": false}}

	change := bson.M{"$addToSet": bson.M{"metas": &meta{Type: fnewmetatype, Id: folderid}}}

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
