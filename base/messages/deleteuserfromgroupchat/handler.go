package main

import (
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type DeleteUserFromGroupChat struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
}

func NewDeleteUserFromGroupChat(mgodb string, mgoAddr string, mgoColl string) (*DeleteUserFromGroupChat, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		return nil, err
	}
	logger.Info("Mongo", "Connected!")

	return &DeleteUserFromGroupChat{mgoSession: mgoSession, mgoColl: mgoSession.DB(mgodb).C(mgoColl)}, nil
}

func (c *DeleteUserFromGroupChat) Close() error {
	c.mgoSession.Close()
	return nil
}

func (conf *DeleteUserFromGroupChat) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.DELETE {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	//AUTH

	chatId := r.Uri.Path
	if chatId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	deletionUserId := r.Uri.Query().Get("deluserid")
	if deletionUserId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	query := bson.M{"_id": chatId, "users": bson.M{"$elemMatch": bson.M{"userid": deletionUserId, "type": bson.M{"$ne": 0}}}} //тут плюсом проверка на то что чел не условный создатель чата

	change := mgo.Change{
		Update:    bson.M{"$pull": bson.M{"users": bson.M{"userid": deletionUserId}}},
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
