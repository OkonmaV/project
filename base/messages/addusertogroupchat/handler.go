package main

import (
	"errors"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type user struct {
	UserId string `bson:"userid"`
	Type   int    `bson:"type"`
	//StartDateTime time.Time `bson:"startdatetime"`
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
	chatId = strings.Trim(chatId, "/")
	if chatId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	addUserId := r.Uri.Query().Get("adduserid")
	if addUserId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	l.Info(addUserId, "AAAAAAAA")

	query := bson.M{"_id": chatId, "type": 2, "users": bson.M{"$elemMatch": bson.M{"userid": userId, "type": 0}}}

	update := bson.M{"$addToSet": bson.M{"users": &user{UserId: addUserId, Type: 1}}}

	changeInfo, err := conf.mgoColl.UpdateAll(query, update)
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
