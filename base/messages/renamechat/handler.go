package main

import (
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type RenameChat struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
}

func NewRenameChat(mgodb string, mgoAddr string, mgoColl string) (*RenameChat, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		return nil, err
	}
	logger.Info("Mongo", "Connected!")

	return &RenameChat{mgoSession: mgoSession, mgoColl: mgoSession.DB(mgodb).C(mgoColl)}, nil
}

func (c *RenameChat) Close() error {
	c.mgoSession.Close()
	return nil
}

func (conf *RenameChat) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

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

	newChatName := r.Uri.Query().Get("newchatname")
	if newChatName == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	query := bson.M{"_id": chatId, "users": bson.M{"$elemMatch": bson.M{"userid": userId, "type": 0}}}

	change := mgo.Change{
		Update:    bson.M{"$set": bson.M{"name": newChatName}},
		Upsert:    true,
		ReturnNew: false,
		Remove:    false,
	}

	if _, err := conf.mgoColl.Find(query).Apply(change, nil); err != nil {
		if err == mgo.ErrNotFound {
			//l.Error("AUTH", errors.New("approved data doesn't match"))
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil

}
