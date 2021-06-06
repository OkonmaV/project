package main

import (
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type DeleteMetaUserFromFolder struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
}

func NewDeleteMetaUserFromFolder(mgodb string, mgoAddr string, mgoColl string) (*DeleteMetaUserFromFolder, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		logger.Error("Mongo conn", err)
		return nil, err
	}
	logger.Info("Mongo", "Connected!")

	mgoCollection := mgoSession.DB(mgodb).C(mgoColl)

	return &DeleteMetaUserFromFolder{mgoSession: mgoSession, mgoColl: mgoCollection}, nil

}

func (conf *DeleteMetaUserFromFolder) Close() error {
	conf.mgoSession.Close()
	return nil
}

func (conf *DeleteMetaUserFromFolder) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.DELETE {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	fid := r.Uri.Path
	fid = strings.Trim(fid, "/")
	deletionMetaId := r.Uri.Query().Get("fdeletemetaid")
	if fid == "" || deletionMetaId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// TODO: AUTH

	query := &bson.M{"_id": fid, "deleted": bson.M{"$exists": false}, "metas": bson.M{"$not": bson.M{"$eq": bson.M{"metaid": deletionMetaId, "metatype": 0}}}}

	change := mgo.Change{
		Update:    bson.M{"$pull": bson.M{"metas": bson.M{"metaid": deletionMetaId}}},
		Upsert:    false,
		ReturnNew: false,
		Remove:    false,
	}

	if _, err := conf.mgoColl.Find(query).Apply(change, nil); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}
	// если ничего не удалило, т.к. метаюзера не было в массиве, то проверки на это нет и все равно 200
	return suckhttp.NewResponse(200, "OK"), nil
}
