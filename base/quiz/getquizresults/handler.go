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

func NewHandler(col *mgo.Collection) (*Handler, error) {
	return &Handler{mgoColl: col}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	query := make(map[string]interface{})
	var selector bson.M
	var err error

	quizId := strings.Trim(r.Uri.Path, "/")
	if quizId != "" {

		query["_id"], err = bson.NewObjectIdFromHex(strings.Trim(r.Uri.Path, "/"))
		if err != nil {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		query := bson.M{"_id": quizId}

		if userId := strings.TrimSpace(r.Uri.Query().Get("userid")); userId != "" { //TODO: take id from cookie?
			selector = bson.M{"usersresults.$": 1}
			query["userresults.userid"] = userId
		}
	} else if entityId := strings.TrimSpace(r.Uri.Query().Get("entityid")); entityId != "" {
		query["entityid"] = entityId
	} else {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var mgoRes []results

	if err := conf.mgoColl.Find(query).Select(selector).All(&mgoRes); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	var body []byte
	var contentType string...

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
