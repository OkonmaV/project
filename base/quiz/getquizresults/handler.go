package main

import (
	"encoding/json"
	"strings"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type Handler struct {
	mgoColl *mgo.Collection
}

type results struct {
	Id       bson.ObjectId       `bson:"_id" json:"id"`
	QuizId   string              `bson:"quizid" json:"quizid"`
	EntityId string              `bson:"entityid" json:"entityid"`
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
	// TODO: AUTH
	query := make(map[string]interface{})

	quizId := strings.Trim(r.Uri.Path, "/")
	if quizId != "" {
		query["quizid"] = quizId
	}

	if userId := strings.TrimSpace(r.Uri.Query().Get("userid")); userId != "" { //TODO: take id from cookie?
		query["userid"] = userId
	}
	if entityId := strings.TrimSpace(r.Uri.Query().Get("entityid")); entityId != "" {
		query["entityid"] = entityId
	}

	if len(query) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var mgoRes []results

	if err := conf.mgoColl.Find(query).All(&mgoRes); err != nil {
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
			l.Error("Marshal", err)
			return suckhttp.NewResponse(500, "Internal server error"), nil
		}
		contentType = "application/json"

	} else {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	return suckhttp.NewResponse(200, "OK").SetBody(body).AddHeader(suckhttp.Content_Type, contentType), nil
}
