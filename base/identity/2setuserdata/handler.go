package main

import (
	"encoding/json"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type user struct {
	//Id       string `bson:"_id" json:"_id"`
	Mail     string `bson:"mail" json:"mail,omitempty"`
	Name     string `bson:"name" json:"name,omitempty"`
	Surname  string `bson:"surname" json:"surname,omitempty"`
	Otch     string `bson:"otch" json:"otch,omitempty"`
	Position string `bson:"position" json:"position,omitempty"`
	MetaId   string `bson:"metaid" json:"metaid,omitempty"`
}

type SetUserData struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
}

func NewSetUserData(mgodb string, mgoAddr string, mgoColl string) (*SetUserData, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		return nil, err
	}
	logger.Info("Mongo", "Connected!")

	return &SetUserData{mgoSession: mgoSession, mgoColl: mgoSession.DB(mgodb).C(mgoColl)}, nil
}

func (c *SetUserData) Close() error {
	c.mgoSession.Close()
	return nil
}

func (conf *SetUserData) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/json") || r.GetMethod() != suckhttp.POST {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	userId := strings.TrimSpace(r.Uri.Query().Get("id"))
	if userId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	if len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	var upsertData map[string]interface{}
	if err := json.Unmarshal(r.Body, &upsertData); err != nil {
		l.Error("Unmarshalling r.Body", err)
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var update bson.M
	if newLogin, ok := upsertData["login"]; ok {
		delete(upsertData, "login")
		update = bson.M{"$set": bson.M{"data": &upsertData}, "$addToSet": bson.M{"logins": newLogin}}
	} else {
		update = bson.M{"$set": bson.M{"data": &upsertData}, "$addToSet": bson.M{"logins": newLogin}}
	}

	changeInfo, err := conf.mgoColl.Upsert(&bson.M{"_id": userId, "deleted": bson.M{"$exists": false}}, update)
	if err != nil {
		return nil, err
	}
	if changeInfo.Matched == 0 {
		return suckhttp.NewResponse(201, "Created"), nil
	}

	return suckhttp.NewResponse(200, "OK"), nil

}
