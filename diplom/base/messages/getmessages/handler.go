package main

import (
	"fmt"
	"strings"
	"text/template"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/roistat/go-clickhouse"
	"go.mongodb.org/mongo-driver/bson"
)

type Handler struct {
	mgoColl         *mgo.Collection
	tokenDecoder    *httpservice.InnerService
	clickhouseConn  *clickhouse.Conn
	clickhouseTable string
	template        *template.Template
}

type chat struct {
	Id    string `bson:"_id"`
	Type  int    `bson:"type"`
	Users []user `bson:"users"`
	Name  string `bson:"name,omitempty"`
}
type user struct {
	UserId        string    `bson:"userid"`
	Type          int       `bson:"type"`
	ChatName      string    `bson:"chatname"`
	StartDateTime time.Time `bson:"startdatetime"`
	//EndDateTime   time.Time `bson:"enddatetime"`
}

type message struct {
	Time    time.Time
	Message string
	Userid  string
}

func NewHandler(mgoColl *mgo.Collection, tokendecoder *httpservice.InnerService, clickhouseConn *clickhouse.Conn, clickhouseTable string) (*Handler, error) {

	return &Handler{mgoColl: mgoColl, tokenDecoder: tokendecoder, clickhouseConn: clickhouseConn, clickhouseTable: clickhouseTable}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	chatId := strings.Trim(r.Uri.Path, "/")
	if chatId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	//TODO FROM TO

	// TODO: AUTH
	koki, ok := r.GetCookie("koki")
	if !ok || len(koki) < 5 {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	tokenDecoderReq, err := conf.tokenDecoder.CreateRequestFrom(suckhttp.GET, suckutils.Concat("/", koki), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	tokenDecoderReq.AddHeader(suckhttp.Accept, "text/plain")
	tokenDecoderResp, err := conf.tokenDecoder.Send(tokenDecoderReq)
	if err != nil {
		l.Error("Send", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if i, t := tokenDecoderResp.GetStatus(); i/100 != 2 {
		l.Debug("Resp from tokendecoder", t)
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	userId := string(tokenDecoderResp.GetBody())
	if userId == "" {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	//

	if err := conf.mgoColl.Find(bson.M{"_id": chatId, "users.userid": userId}).Select(bson.M{"_id": 1}).One(nil); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), err
		}
		return nil, err
	}

	query := clickhouse.NewQuery(suckutils.Concat("SELECT time,message,userid,chatid FROM ", conf.clickhouseTable, " WHERE chatid = '", chatId, "'"))
	iter := query.Iter(conf.clickhouseConn)

	messages := []message{}
	var i int = 0
	var (
		time   time.Time
		text   string
		userid string
	)
	for iter.Scan(&time, &text, &userid) {
		messages = append(messages, message{Time: time, Message: text, Userid: userid}) //get usernames from mongo
		i++
	}
	fmt.Println(messages)
	if iter.Error() != nil {
		l.Error("Clickhouse", iter.Error())
	}
	// var body []byte
	// var contentType string
	// if len(mgoRes) != 0 {
	// 	for i, chatt := range mgoRes {

	// 		if chatt.Type == 1 {
	// 			if len(chatt.Users) != 2 {
	// 				l.Error("Chat", errors.New("chattype unmatches with len(chatusers)"))
	// 				mgoRes[i] = chat{} //????????
	// 				continue           //??????????
	// 			}

	// 			if chatt.Users[0].UserId == userId {
	// 				mgoRes[i].Name = chatt.Users[0].ChatName
	// 			} else {
	// 				mgoRes[i].Name = chatt.Users[1].ChatName
	// 			}
	// 		}
	// 	}
	// 	buf := bytes.NewBuffer(body)
	// 	err := conf.template.Execute(buf, mgoRes)
	// 	if err != nil {
	// 		l.Error("Template execution", err)
	// 		return suckhttp.NewResponse(500, "Internal server error"), err
	// 	}
	// 	body = buf.Bytes()
	// 	contentType = "text/html"
	// }

	//return suckhttp.NewResponse(200, "OK").SetBody(body).AddHeader(suckhttp.Content_Type, contentType), nil
	return suckhttp.NewResponse(200, "OK"), nil
}
