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

type Handler struct {
	mgoColl         *mgo.Collection
	clickhouseConn  *clickhouse.Conn
	clickhouseTable string
}
type chatInfo struct {
	Id    string        `bson:"_id"`
	Users []interface{} `bson:"users"`
	Name  string        `bson:"name"`
	Type  int           `bson:"type"`
}

func NewHandler(mgoColl *mgo.Collection, clickhouseConn *clickhouse.Conn, clickhouseTable string) (*Handler, error) {

	return &Handler{mgoColl: mgoColl, clickhouseConn: clickhouseConn, clickhouseTable: clickhouseTable}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "text/plain") {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	chatId := strings.Trim(r.Uri.Path, "/")
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
		clickhouse.Row{time.Now(), chatId, userId, message, 1})
	if err != nil {
		return nil, err
	}

	if err = query.Exec(conf.clickhouseConn); err != nil {
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
