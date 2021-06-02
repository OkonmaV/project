package main

import (
	"strconv"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/rs/xid"
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

type CreateChat struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
}

func NewCreateChat(mgodb string, mgoAddr string, mgoColl string) (*CreateChat, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		return nil, err
	}
	logger.Info("Mongo", "Connected!")

	return &CreateChat{mgoSession: mgoSession, mgoColl: mgoSession.DB(mgodb).C(mgoColl)}, nil
}

func (c *CreateChat) Close() error {
	c.mgoSession.Close()
	return nil
}

func (conf *CreateChat) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST { //  КАКОЙ МЕТОД?
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	//AUTH
	userId := "testOwner"
	//

	chatType, err := strconv.Atoi(r.Uri.Query().Get("chattype"))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var query bson.M
	var update bson.M
	switch chatType {
	case 1: //tet-a-tet

		withUserId := r.Uri.Query().Get("withid")
		if withUserId == "" {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}

		//query = bson.M{"type": chatType, "users": bson.M{"$all": []bson.M{{"$elemMatch": bson.M{"userid": userId}}, {"$elemMatch": bson.M{"userid": withUserId}}}}}
		query = bson.M{"type": chatType, "$or": []bson.M{{"users.0.userid": userId, "users.1.userid": withUserId}, {"users.0.userid": withUserId, "users.1.userid": userId}}}
		update = bson.M{"$setOnInsert": &chat{Id: xid.New().String(), Type: chatType, Users: []user{{UserId: userId, Type: 0, StartDateTime: time.Now()}, {UserId: withUserId, Type: 0, StartDateTime: time.Now()}}}}

	case 2: //group

		chatName := r.Uri.Query().Get("chatname")
		if chatName == "" {
			chatName = "Group chat"
		}

		//query = bson.M{"type": chatType, "users": bson.M{"$elemMatch": bson.M{"userid": userId, "type": 0}}, "name": chatName}
		query = bson.M{"type": chatType, "users.0.userid": "userId", "users.0.type": 0, "name": chatName}
		update = bson.M{"$setOnInsert": &chat{Id: xid.New().String(), Type: chatType, Users: []user{{UserId: userId, Type: 0, StartDateTime: time.Now()}}}}

	default:
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	change := mgo.Change{
		Update:    update,
		Upsert:    true,
		ReturnNew: true,
		Remove:    false,
	}

	var mgoRes map[string]string
	changeInfo, err := conf.mgoColl.Find(query).Select(bson.M{"_id": 1}).Apply(change, &mgoRes)
	if err != nil {
		return nil, err
	}
	if changeInfo.Matched == 0 {
		return suckhttp.NewResponse(201, "Created").SetBody([]byte(mgoRes["_id"])), nil
	} else {
		return suckhttp.NewResponse(200, "OK").SetBody([]byte(mgoRes["_id"])), nil
	}

}
