package main

import (
	"errors"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/suckhttp"
	"go.mongodb.org/mongo-driver/bson"
)

type chat struct {
	Id    string `bson:"_id"`
	Type  int    `bson:"type"`
	Users []user `bson:"users"`
	Name  string `bson:"name,omitempty"`
}
type user struct {
	UserId        string    `bson:"userid"`
	Type          int       `bson:"type"`
	StartDateTime time.Time `bson:"startdatetime"`
	EndDateTime   time.Time `bson:"enddatetime"`
}

type DeleteChat struct {
	mgoSession        *mgo.Session
	mgoColl           *mgo.Collection
	mgoCollForDeleted *mgo.Collection
}

func NewDeleteChat(mgodb string, mgoAddr string, mgoColl string, mgoCollForDeleted string) (*DeleteChat, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		return nil, err
	}
	logger.Info("Mongo", "Connected!")

	return &DeleteChat{mgoSession: mgoSession, mgoColl: mgoSession.DB(mgodb).C(mgoColl), mgoCollForDeleted: mgoSession.DB(mgodb).C(mgoCollForDeleted)}, nil
}

func (c *DeleteChat) Close() error {
	c.mgoSession.Close()
	return nil
}

func (conf *DeleteChat) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.DELETE {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	//AUTH
	userId := "testOwner"
	//

	chatId := r.Uri.Path
	if chatId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	//select
	query := bson.M{"_id": chatId, "users": bson.M{"$elemMatch": bson.M{"userid": userId, "type": 0}}}

	change := mgo.Change{
		Update:    bson.M{"$set": bson.M{"deletion": 1}},
		Upsert:    true,
		ReturnNew: false,
		Remove:    false,
	}

	if _, err := conf.mgoColl.Find(query).Apply(change, nil); err != nil {
		if err == mgo.ErrNotFound {
			l.Error("AUTH", errors.New("approved data doesn't match"))
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	//upsert
	if _, err := conf.mgoCollForDeleted.UpsertId(chatId, nil); err != nil {
		return nil, err
	}

	//delete
	if err := conf.mgoColl.RemoveId(chatId); err != nil {
		if err == mgo.ErrNotFound {
			l.Error("Removing from chats", errors.New("trying remove already removed chat"))
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}
	return suckhttp.NewResponse(200, "OK"), nil

}
