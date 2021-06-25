package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io/ioutil"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type Handler struct {
	mgoColl  *mgo.Collection
	template *template.Template
}
type quiz struct {
	Id        bson.ObjectId       `bson:"_id" json:"quizid"`
	Name      string              `bson:"name" json:"quizname"`
	Questions map[string]question `bson:"questions" json:"questions"`
	CreatorId string              `bson:"creatorid" json:"creatorid"`
}

type question struct {
	Type     int               `bson:"question_type" json:"question_type"`
	Position int               `bson:"question_position" json:"question_position"`
	Text     string            `bson:"question_text" json:"question_text"`
	Answers  map[string]string `bson:"question_answers" json:"question_answers"`
}

func NewHandler(col *mgo.Collection) (*Handler, error) {
	templData, err := ioutil.ReadFile("index.html")
	if err != nil {
		return nil, err
	}

	templ, err := template.New("index").Parse(string(templData))
	if err != nil {
		return nil, err
	}

	return &Handler{mgoColl: col, template: templ}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	quizId, err := bson.NewObjectIdFromHex(strings.Trim(r.Uri.Path, "/"))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// AUTH
	if foo, ok := r.GetCookie("koki"); !ok || len(foo) < 5 {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	//

	var mgoRes quiz

	if err := conf.mgoColl.Find(bson.M{"_id": quizId, "deleted": bson.M{"$exists": false}}).One(&mgoRes); err != nil {
		if err == mgo.ErrNotFound {
			l.Debug("Mongo", "not found")
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	var body []byte
	var contentType string

	if len(mgoRes.Questions) != 0 {
		if strings.Contains(r.GetHeader(suckhttp.Accept), "text/html") {
			buf := bytes.NewBuffer(body)
			err := conf.template.Execute(buf, mgoRes.Questions)
			if err != nil {
				l.Error("Template execution", err)
				return suckhttp.NewResponse(500, "Internal server error"), err
			}
			body = buf.Bytes()
			contentType = "text/html"
		} else if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {
			if body, err = json.Marshal(mgoRes); err != nil {
				l.Error("Marshal", err)
				return suckhttp.NewResponse(500, "Internal server error"), nil
			}
			contentType = "application/json"
		} else {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
	}

	return suckhttp.NewResponse(200, "OK").SetBody(body).AddHeader(suckhttp.Content_Type, contentType), nil
}
