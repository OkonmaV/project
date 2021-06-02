package main

import (
	"strings"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/roistat/go-clickhouse"
)

type SendMessage struct {
	mgoSession      *mgo.Session
	mgoColl         *mgo.Collection
	clickhouseConn  *clickhouse.Conn
	clickhouseTable string
}
type chatInfo struct {
	Id    string   `bson:"_id"`
	Users []string `bson:"users"`
	Name  string   `bson:"name"`
	Type  int      `bson:"type"`
}

func NewSendMessage(mgoAddr string, mgoColl string, clickhouseAddr string, clickhouseTable string) (*SendMessage, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		return nil, err
	}
	logger.Info("Mongo", "Connected!")
	mgoCollection := mgoSession.DB("main").C(mgoColl)

	chConn := clickhouse.NewConn(clickhouseAddr, clickhouse.NewHttpTransport())
	//"CREATE TABLE IF NOT EXISTS main.chats (time DateTime,chatID UUID,user String,text String) ENGINE = MergeTree() ORDER BY tuple()"
	err = chConn.Ping()
	if err != nil {
		return nil, err
	}
	logger.Info("Clickhouse", "Connected!")
	return &SendMessage{mgoSession: mgoSession, mgoColl: mgoCollection, clickhouseConn: chConn, clickhouseTable: clickhouseTable}, nil

}

func (conf *SendMessage) Close() error {
	conf.mgoSession.Close()
	return nil
}

func (conf *SendMessage) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "text/plain") {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	chatId := r.Uri.Path
	if chatId == "" {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}
	message := strings.TrimSpace(string(r.Body))
	if message == "" {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	// TODO: AUTH?
	userId := "testUser"
	//

	if err := conf.mgoColl.Find(bson.M{"Id": chatId, "users.userid": userId}).Select(bson.M{"_id": 1}).One(nil); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), err
		}
		return nil, err
	}

	query, err := clickhouse.BuildInsert(conf.clickhouseTable,
		clickhouse.Columns{"time", "chatID", "user", "message", "type"},
		clickhouse.Row{time.Now(), chatId, userId, message, 0})
	if err != nil {
		return nil, err
	}
	err = query.Exec(conf.clickhouseConn)
	if err != nil {
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
