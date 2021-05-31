package main

import (
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

	if _, err = conf.mgoColl.Find(query).Apply(change, nil); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
