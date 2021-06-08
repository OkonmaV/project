package main

import (
	"os"
	"strconv"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type Handler struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
}

//quiz
type quiz struct {
	Id        string              `bson:"_id"`
	Name      string              `bson:"name"`
	Questions map[string]question `bson:"questions"`
	CreatorId string              `bson:"creatorid"`
}

type question struct {
	Type     int               `bson:"question_type"`
	Position int               `bson:"question_position"`
	Text     string            `bson:"question_text"`
	Answers  map[string]string `bson:"question_answers"`
}

//

func NewHandler(mgodb string, mgoAddr string, mgoColl string) (*Handler, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		logger.Error("Mongo conn", err)
		return nil, err
	}
	logger.Info("Mongo", "Connected!")
	mgoCollection := mgoSession.DB(mgodb).C(mgoColl)

	return &Handler{mgoSession: mgoSession, mgoColl: mgoCollection}, nil

}

func (conf *Handler) Close() error {
	conf.mgoSession.Close()
	return nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	quizId := strings.Trim(r.Uri.Path, "/")
	if quizId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	fileName := r.Uri.Query().Get("filename")
	if fileName == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	// TODO: AUTH
	//userId := "testuserid"
	//

	fileData, err := os.ReadFile(suckutils.ConcatTwo(fileName, ".txt"))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	if len(fileData) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	lines := strings.Split(strings.TrimSpace(string(fileData)), "\n")

	questions := make([]*question, 0)
	var curquestion *question
	for _, line := range lines {
		if line == "" { // commit current question
			//TODO new question
			questions = append(questions, curquestion)
			curquestion = nil
		} else if curquestion == nil { // new question
			space_position := strings.Index(line, " ")
			if space_position == -1 {
				//TODO error
			}
			t, err := strconv.Atoi(line[:space_position])
			if err != nil {
				// TODO error
			}
			curquestion = &question{Type: t, Text: strings.TrimSpace(line[space_position+1:]), Answers: make(map[string]string)}
		} else { // answer
			curquestion.Answers[bson.NewObjectId().Hex()] = strings.TrimSpace(line)
		}
	}

	//TODO save

	return suckhttp.NewResponse(200, "OK"), nil
}
