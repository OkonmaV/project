package main

import (
	"net/url"
	"strings"
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

func getRandId() string {
	return xid.New().String()
}

func (conf *CreateFolder) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad Request"), err
	}

	froot := formValues.Get("frootid")
	fname := formValues.Get("fname")
	if froot == "" || fname == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// TODO: AUTH

	// TODO: get metauser
	metaid := "randmetaid"
	//

	// проверяем корень??? удел AUTH???
	query := &bson.M{"_id": froot, "deleted": bson.M{"$exists": false}}
	var foo interface{}

	err = conf.mgoColl.Find(query).One(&foo)
	if err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}
	//

	err = conf.mgoColl.Insert(&folder{Id: getRandId(), RootsId: []string{froot}, Name: fname, Metas: []meta{{Type: 0, Id: metaid}}, LastModified: time.Now()})
	if err != nil {
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
