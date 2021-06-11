package main

import (
	"net/url"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type Handler struct {
	mgoColl *mgo.Collection
	authGet *httpservice.Authorizer
	authSet *httpservice.InnerService
}
type quiz struct {
	Id        bson.ObjectId       `bson:"_id"`
	Name      string              `bson:"name"`
	Questions map[string]question `bson:"questions"`
	CreatorId string              `bson:"creatorid"`
}

type question struct {
	Type     int               `bson:"question_type"`
	Position int               `bson:"question_position"`
	Text     string            `bson:"question_text"`
	Answers  map[string]string `bson:"answers"`
}

func NewHandler(col *mgo.Collection, authGet *httpservice.InnerService, authSet *httpservice.InnerService, tokendecoder *httpservice.InnerService) (*Handler, error) {
	authorizerGet, err := httpservice.NewAuthorizer(thisServiceName, authGet, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{mgoColl: col, authGet: authorizerGet, authSet: authSet}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.PUT || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	quizName := formValues.Get("quizname")
	if quizName == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	k, creatorId, err := conf.authGet.GetAccess(r, l, "createquiz", 1)
	if err != nil {
		return nil, err
	}
	if !k {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	// TODO:HOW TO SET?

	quizId := bson.NewObjectId()
	//нет проверки по имени

	if err = conf.mgoColl.Insert(&quiz{Id: quizId, Name: quizName, CreatorId: creatorId}); err != nil {
		return nil, err
	}

	authSetReq, err := conf.authSet.CreateRequestFrom(suckhttp.POST, suckutils.Concat("/", creatorId, ".", quizId.Hex(), "?perm=1"), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	authSetResp, err := conf.authSet.Send(authSetReq)
	if err != nil {
		l.Error("Send", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if i, t := authSetResp.GetStatus(); i/100 != 2 {
		l.Debug("Resp from auth", t)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	resp := suckhttp.NewResponse(201, "Created")
	if strings.Contains(r.GetHeader(suckhttp.Accept), "text/plain") {
		resp.SetBody([]byte(quizId.Hex())).AddHeader(suckhttp.Content_Type, "text/plain")
	}
	return resp, nil
}
