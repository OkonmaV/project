package main

import (
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type DeleteFolder struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
}

func NewDeleteFolder(mgodb string, mgoAddr string, mgoColl string) (*DeleteFolder, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		logger.Error("Mongo conn", err)
		return nil, err
	}
	logger.Info("Mongo", "Connected!")

	mgoCollection := mgoSession.DB(mgodb).C(mgoColl)

	return &DeleteFolder{mgoSession: mgoSession, mgoColl: mgoCollection}, nil

}

func (conf *DeleteFolder) Close() error {
	conf.mgoSession.Close()
	return nil
}

func (conf *DeleteFolder) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.DELETE {
		return suckhttp.NewResponse(405, "Method not allowed"), nil
	}

	fid := r.Uri.Path //??????????????????????????????????????????????????????????????????????????????????????????????????
	if fid == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// TODO: AUTH

	// TODO: get metauser
	metaid := "randmetaid"
	//

	query := &bson.M{"_id": fid, "deleted": bson.M{"$exists": false}}

	change := mgo.Change{
		Update:    bson.M{"$set": bson.M{"deleted.by": metaid}, "$currentDate": bson.M{"deleted.date": true}},
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
