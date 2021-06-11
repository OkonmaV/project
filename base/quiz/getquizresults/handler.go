package main

import (
	"encoding/json"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type Handler struct {
	mgoColl *mgo.Collection
	auth    *httpservice.Authorizer
}

type results struct {
	QuizId       bson.ObjectId `bson:"_id" json:"quizid"`
	EntityId     string        `bson:"entityid" json:"entityid"`
	Usersresults []userresult  `bson:"usersresults" json:"usersresults"`
}

type userresult struct {
	UserId      string        `bson:"userid" json:"userid"`
	UserAnswers []useranswers `bson:"useranswers" json:"useranswers"`
	Datetine    time.Time     `bson:"datetime" json:"datetime"`
}

type useranswers struct {
	UserId   string              `bson:"userid" json:"userid"`
	Answers  map[string][]string `bson:"answers" json:"answers"`
	Datetime time.Time           `bson:"datetime" json:"datetime"`
}

func NewHandler(col *mgo.Collection, auth *httpservice.InnerService, tokendecoder *httpservice.InnerService) (*Handler, error) {
	authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{mgoColl: col, auth: authorizer}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	quizId, err := bson.NewObjectIdFromHex(strings.Trim(r.Uri.Path, "/"))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	userId := strings.TrimSpace(r.Uri.Query().Get("userid"))

	// TODO: AUTH
	// k, _, err := conf.auth.GetAccess(r, l, "getquizresults", 1)
	// if err != nil {
	// 	return nil, err
	// }
	// if !k {
	// 	return suckhttp.NewResponse(403, "Forbidden"), nil
	// }

	var mgoRes results
	var selector bson.M
	query := bson.M{"_id": quizId}

	if userId != "" {
		selector = bson.M{"usersresults.$": 1}
		query["userresults.userid"] = userId
	}
	if err := conf.mgoColl.Find(bson.M{"_id": quizId}).Select(selector).One(&mgoRes); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	var body []byte
	var contentType string

	if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {
		var err error
		body, err = json.Marshal(mgoRes)
		if err != nil {
			l.Error("Marshalling mongo responce", err)
			return suckhttp.NewResponse(500, "Internal server error"), nil
		}
		contentType = "application/json"
	}

	return suckhttp.NewResponse(200, "OK").SetBody(body).AddHeader(suckhttp.Content_Type, contentType), nil
}
