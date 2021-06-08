package main

import (
	"os"
	"strconv"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/rs/xid"
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

	foo := strings.Split(string(fileData), "\n\n")

	lines := make([][]string, len(foo))
	for i, bar := range foo {
		lines[i] = strings.Split(bar, "\n")
	}

	questions := make(map[string]question)

	for i, bar := range lines {
		if len(bar) < 3 {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		if bar[1] == "3" {

			questions[xid.New().String()] = question{Type: 3, Position: i, Text: bar[2]}
			continue

		} else if bar[1] == "1" || bar[1] == "2" {

			answers := make(map[string]string)
			for i := 2; i < len(bar); i++ {
				answers[xid.New().String()] = bar[i]
			}
			h, _ := strconv.Atoi(bar[1])
			questions[xid.New().String()] = question{Type: 3, Position: i, Text: bar[2]}
		} else {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}

	}

	// file, err := os.Open(suckutils.ConcatTwo(fileName, ".txt"))
	// if err != nil {
	// 	return suckhttp.NewResponse(400, "Bad request"), nil
	// }

	// scaner := bufio.NewScanner(file)

	// for scaner.Scan(){

	// }

	return suckhttp.NewResponse(200, "OK"), nil
}
