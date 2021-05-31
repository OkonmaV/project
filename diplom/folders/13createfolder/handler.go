package main

import (
	"errors"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/rs/xid"
)

type CreateFolder struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
}
type folder struct {
	Id           string    `bson:"_id"`
	RootsId      []string  `bson:"rootsid"`
	Name         string    `bson:"name"`
	Metas        []meta    `bson:"metas"`
	LastModified time.Time `bson:"lastmodified"`
}

type meta struct {
	Type int    `bson:"metatype"`
	Id   string `bson:"metaid"`
}

func NewCreateFolder(mgodb string, mgoAddr string, mgoColl string) (*CreateFolder, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		logger.Error("Mongo conn", err)
		return nil, err
	}
	logger.Info("Mongo", "Connected!")
	mgoCollection := mgoSession.DB(mgodb).C(mgoColl)

	return &CreateFolder{mgoSession: mgoSession, mgoColl: mgoCollection}, nil

}

func (conf *CreateFolder) Close() error {
	conf.mgoSession.Close()
	return nil
}

func (conf *CreateFolder) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.PUT {
		return suckhttp.NewResponse(405, "Method not allowed"), nil
	}

	folderRootId := r.Uri.Path //?????????????????????????????????????????????????????????????????????
	folderName := r.Uri.Query().Get("name")
	if folderName == "" || folderRootId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// TODO: AUTH

	// TODO: get metauser
	metaid := "randmetaid"
	//

	// check root
	query := &bson.M{"_id": folderRootId, "deleted": bson.M{"$exists": false}}
	var foo interface{}

	if err := conf.mgoColl.Find(query).Select(bson.M{"_id": 1}).One(&foo); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}
	//

	newfolderId := xid.New()
	if newfolderId.IsNil() {
		return nil, errors.New("get new rand id returned nil")
	}
	query = &bson.M{"name": folderName, "rootsid": folderRootId, "deleted": bson.M{"$exists": false}}
	change := &bson.M{"$setOnInsert": &folder{Id: newfolderId.String(), RootsId: []string{folderRootId}, Name: folderName, Metas: []meta{{Type: 0, Id: metaid}}}}

	changeInfo, err := conf.mgoColl.Upsert(query, change)
	if err != nil {
		return nil, err
	}
	if changeInfo.Matched != 0 {
		return suckhttp.NewResponse(409, "Conflict"), nil
	}

	return suckhttp.NewResponse(201, "Created").SetBody(newfolderId.Bytes()), nil
}
