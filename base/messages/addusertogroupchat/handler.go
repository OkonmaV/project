package main

import (
	"errors"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type user struct {
	UserId        string    `bson:"userid"`
	Type          int       `bson:"type"`
	StartDateTime time.Time `bson:"startdatetime"`
	EndDateTime   time.Time `bson:"enddatetime"`
}

type AddUserToGroupChat struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
}

func NewAddUserToGroupChat(mgodb string, mgoAddr string, mgoColl string) (*AddUserToGroupChat, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		return nil, err
	}
	logger.Info("Mongo", "Connected!")

	return &AddUserToGroupChat{mgoSession: mgoSession, mgoColl: mgoSession.DB(mgodb).C(mgoColl)}, nil
}

func (c *AddUserToGroupChat) Close() error {
	c.mgoSession.Close()
	return nil
}

func (conf *AddUserToGroupChat) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.HttpMethod("PATCH") {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	//AUTH
	userId := "testOwner"
	//

	chatId := r.Uri.Path
	if chatId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	addUserId := r.Uri.Query().Get("adduserid")
	if addUserId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	query := bson.M{"_id": chatId, "users": bson.M{"$elemMatch": bson.M{"userid": userId, "type": 0}}}

	change := mgo.Change{
		Update:    bson.M{"$addToSet": bson.M{"user": &user{UserId: addUserId, Type: 1}}},
		Upsert:    true,
		ReturnNew: false,
		Remove:    false,
	}

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
