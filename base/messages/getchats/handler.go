package main

import (
	"bytes"
	"errors"
	"project/base/messages/repo"
	"text/template"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"go.mongodb.org/mongo-driver/bson"
)

type Handler struct {
	mgoColl      *mgo.Collection
	tokenDecoder *httpservice.InnerService
	template     *template.Template
}

func NewHandler(col *mgo.Collection, tokendecoder *httpservice.InnerService, template *template.Template) (*Handler, error) {

	return &Handler{mgoColl: col, tokenDecoder: tokendecoder, template: template}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		l.Debug("Request", "not GET")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

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
	chats := []repo.Chat{}

	if err := conf.mgoColl.Find(bson.M{"users.userid:": userId}).All(&chats); err != nil {
		return nil, err
	}

	var body []byte
	var contentType string
	if len(chats) != 0 {
		for i, chat := range chats {

			if chat.Type == 1 {
				if len(chat.Users) != 2 {
					l.Error("Chat", errors.New("chattype unmatches with len(chatusers)"))
					chats[i] = repo.Chat{} //????????
					continue               //??????????
				}

				if chat.Users[0].UserId == userId {
					chats[i].Name = chat.Users[0].ChatName
				} else {
					chats[i].Name = chat.Users[1].ChatName
				}
			}
		}
		buf := bytes.NewBuffer(body)
		err := conf.template.Execute(buf, chats)
		if err != nil {
			return suckhttp.NewResponse(500, "Internal server error"), err
		}
		body = buf.Bytes()
		contentType = "text/html"
	} // what else???

	return suckhttp.NewResponse(200, "OK").SetBody(body).AddHeader(suckhttp.Content_Type, contentType), nil

}
