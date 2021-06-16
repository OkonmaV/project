package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime"
	"net/url"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/nguyenthenguyen/docx"
)

type Handler struct {
	tokenDecoder   *httpservice.InnerService
	getUserData    *httpservice.InnerService
	getQuizResults *httpservice.InnerService
}

type cookieData struct {
	UserId string `json:"Login"`
	Role   int    `json:"role"`
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

type results1 struct {
	Id       bson.ObjectId       `bson:"_id" json:"id"`
	QuizId   string              `bson:"quizid" json:"quizid"`
	EntityId string              `bson:"entityid" json:"entityid"`
	UserId   string              `bson:"userid" json:"userid"`
	Answers  map[string][]string `bson:"answers" json:"answers"`
	Datetime time.Time           `bson:"datetime" json:"datetime"`
}

type claims struct {
	MetaId  string `json:"metaid"`
	Role    int    `json:"role"`
	Surname string `json:"surname"`
	Name    string `json:"name"`
	Group string `json:"group"`
}

func NewHandler(getuserdata, getquizresults, tokendecoder *httpservice.InnerService) (*Handler, error) {
	return &Handler{tokenDecoder: tokendecoder, getUserData: getuserdata, getQuizResults: getquizresults}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	quizId := formValues.Get("quizid")
	if quizId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	folderId := formValues.Get("folderid")
	if folderId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	userId := formValues.Get("userid")
	if userId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// check role
	koki, ok := r.GetCookie("koki")
	if !ok || len(koki) < 5 {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	tokenDecoderReq, err := conf.tokenDecoder.CreateRequestFrom(suckhttp.GET, suckutils.Concat("/", koki), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	tokenDecoderReq.AddHeader(suckhttp.Accept, "application/json")
	tokenDecoderResp, err := conf.tokenDecoder.Send(tokenDecoderReq)
	if err != nil {
		l.Error("Send", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if i, t := tokenDecoderResp.GetStatus(); i/100 != 2 {
		l.Debug("Resp from tokendecoder", t)
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	if len(tokenDecoderResp.GetBody()) == 0 {
		l.Debug("Resp from tokendecoder", "empty body")
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	userData := &cookieData{}

	if err = json.Unmarshal(tokenDecoderResp.GetBody(), userData); err != nil {
		l.Error("Unmarshal", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if userData.Role < 2 || userData.UserId == "" {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	//

	getUserDataReq, err := conf.getUserData.CreateRequestFrom(suckhttp.GET, suckutils.ConcatThree("/", userId, "?fields=metaid,role,surname,name,group"), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	getUserDataReq.AddHeader(suckhttp.Accept, "application/json")
	getUserDataResp, err := conf.getUserData.Send(getUserDataReq)
	if err != nil {
		l.Error("Send", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	if i, t := getUserDataResp.GetStatus(); i != 200 {
		l.Debug("Resp from getuserdata", t)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if len(getUserDataResp.GetBody()) == 0 {
		l.Error("Resp from getuserdata", errors.New("empty body at 200"))
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	var clms claims
	if err := json.Unmarshal(getUserDataResp.GetBody(), &clms); err != nil {
		l.Error("Unmarshal", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	// Get Quiz Results For UserId & FolderId

	quizResultsReq, _ := conf.getQuizResults.CreateRequestFrom(suckhttp.GET, suckutils.Concat("/", quizId, "?userid=", userId, "&entityid=", folderId), r)
	quizResultsReq.AddHeader(suckhttp.Accept, "application/json")
	resp, err := conf.getQuizResults.Send(quizResultsReq)
	if err != nil {
		l.Error("getQuizResults", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	quizResults := &results{}

	if err = json.Unmarshal(resp.GetBody(), quizResults); err != nil {
		l.Error("UnmarshalQuizResult", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	// Generating protocol

	doc, err := docx.ReadDocxFile("protocol.docx")
	if err != nil {
		l.Error("ReadDOCX", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	edit := doc.Editable()
	edit.Replace("{группа}", clms., -1)
	doc.Close()
	buf := &bytes.Buffer{}
	err = edit.Write(buf)
	if err != nil {
		l.Error("WriteDOCX", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	res := suckhttp.NewResponse(200, "OK")
	res.AddHeader(suckhttp.Content_Type, mime.TypeByExtension(".docx"))
	res.SetBody(buf.Bytes())
	return res, nil
}
