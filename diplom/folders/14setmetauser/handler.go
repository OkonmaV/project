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
	mgoSession       *mgo.Session
	mgoColl          *mgo.Collection
	mgoCollMetausers *mgo.Collection
}

type meta struct {
	Type int    `bson:"metatype"`
	Id   string `bson:"metaid"`
}

func NewSetMetaUser(mgodb string, mgoAddr string, mgoColl string, mgoCollMetausers string) (*SetMetaUser, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		logger.Error("Mongo conn", err)
		return nil, err
	}
	logger.Info("Mongo", "Connected!")
	mgoCollection := mgoSession.DB(mgodb).C(mgoColl)
	mgoCollectionMetausers := mgoSession.DB(mgodb).C(mgoCollMetausers)

	return &SetMetaUser{mgoSession: mgoSession, mgoColl: mgoCollection, mgoCollMetausers: mgoCollectionMetausers}, nil

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
	folderid = strings.Trim(folderid, "/")
	fnewmeta := strings.TrimSpace(r.Uri.Query().Get("fnewmetaid"))
	if folderid == "" || fnewmeta == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	fnewmetatype, err := strconv.Atoi(r.Uri.Query().Get("fnewmetatype"))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	// TODO: AUTH

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
