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
	getFolders     *httpservice.InnerService
}

type cookieData struct {
	UserId string `json:"Login"`
	Role   int    `json:"role"`
}

// QUIZ RESULT
type results struct {
	Id       bson.ObjectId       `bson:"_id" json:"id"`
	QuizId   string              `bson:"quizid" json:"quizid"`
	EntityId string              `bson:"entityid" json:"entityid"`
	UserId   string              `bson:"userid" json:"userid"`
	Answers  map[string][]string `bson:"answers" json:"answers"`
	Datetime time.Time           `bson:"datetime" json:"datetime"`
}

//
// FOLDER
type folder struct {
	Id      string        `bson:"_id"`
	RootsId []string      `bson:"rootsid"`
	Name    string        `bson:"name"`
	Type    int           `bson:"type"`
	Metas   []interface{} `bson:"metas"`
}

//
// USERDATA
type userData struct {
	//Id       string `bson:"_id" json:"_id"`
	Mail     string `bson:"mail" json:"mail,omitempty"`
	Name     string `bson:"name" json:"name,omitempty"`
	Surname  string `bson:"surname" json:"surname,omitempty"`
	Otch     string `bson:"otch" json:"otch,omitempty"`
	GroupId  string `bson:"groupid" json:"groupid,omitempty"`
	MetaId   string `bson:"metaid" json:"metaid,omitempty"`
	FolderId string `bson:"folderid" json:"folderid,omitempty"`
}

//
type claims struct {
	MetaId  string `json:"metaid"`
	Role    int    `json:"role"`
	Surname string `json:"surname"`
	Name    string `json:"name"`
	Group   string `json:"group"`
}

func NewHandler(tokendecoder, getuserdata, getquizresults, getfolders *httpservice.InnerService) (*Handler, error) {
	return &Handler{tokenDecoder: tokendecoder, getUserData: getuserdata, getQuizResults: getquizresults, getFolders: getfolders}, nil
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
	folderId := formValues.Get("folderid") // entity id
	if folderId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	userId := formValues.Get("userid") // student's id
	if userId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// AUTH
	koki, ok := r.GetCookie("koki")
	if !ok || len(koki) < 5 {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	//

	// Get Quiz Results For UserId & FolderId

	quizResultsReq, err := conf.getQuizResults.CreateRequestFrom(suckhttp.GET, suckutils.Concat("/", quizId, "?entityid=", folderId), r)
	if err != nil {
		l.Error("CreateRequiesFrom quizRes", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	quizResultsReq.AddHeader(suckhttp.Accept, "application/json")
	quizResultsResp, err := conf.getQuizResults.Send(quizResultsReq)
	if err != nil {
		l.Error("getQuizResults", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	if i, t := quizResultsResp.GetStatus(); i/100 != 2 {
		l.Debug("Resp from getquizresults", t)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	if len(quizResultsResp.GetBody()) == 0 {
		l.Error("QuizResults responce", errors.New("empty body at 200"))
		return suckhttp.NewResponse(400, "Bad requiest"), nil
	}

	quizResults := []results{}

	if err = json.Unmarshal(quizResultsResp.GetBody(), &quizResults); err != nil {
		l.Error("UnmarshalQuizResult", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	// if len(quizResults)==0{
	// 	l.Error("QuizResults",errors.New(""))
	// }

	// GET STUDENT'S DATA
	var studentUserData userData

	getUserDataReq, err := conf.getUserData.CreateRequestFrom(suckhttp.GET, suckutils.ConcatThree("/", userId, "?fields=surname,name,otch,groupid,folderid"), r) // no metaid??
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

	if i, t := getUserDataResp.GetStatus(); i/100 != 2 {
		l.Debug("Resp from getuserdata", t)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if len(getUserDataResp.GetBody()) == 0 {
		l.Error("Resp from getuserdata", errors.New("empty body at 200"))
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if err := json.Unmarshal(getUserDataResp.GetBody(), &studentUserData); err != nil {
		l.Error("Unmarshal", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	//

	// GET GAK'S DATA
	var gakUserData []*userData

	for _, userResult := range quizResults {
		getUserDataReq, err := conf.getUserData.CreateRequestFrom(suckhttp.GET, suckutils.ConcatThree("/", userResult.UserId, "?fields=surname,name,otch"), r)
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

		if i, t := getUserDataResp.GetStatus(); i/100 != 2 {
			l.Debug("Resp from getuserdata", t)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		if len(getUserDataResp.GetBody()) == 0 {
			l.Error("Resp from getuserdata", errors.New("empty body at 200"))
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		if err := json.Unmarshal(getUserDataResp.GetBody(), &gakUserData); err != nil {
			l.Error("Unmarshal", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
	}
	//

	// Generating protocol

	doc, err := docx.ReadDocxFile("protocol.docx")
	if err != nil {
		l.Error("ReadDOCX", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	edit := doc.Editable()
	edit.Replace("{группа}", clms, -1)
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
