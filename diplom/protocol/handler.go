package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime"
	"net/url"
	"strconv"
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
	Id         string   `bson:"_id" json:"_id"`
	RootsId    []string `bson:"rootsid" json:"rootsid"`
	Name       string   `bson:"name" json:"name"`
	Metas      []meta   `bson:"metas" json:"metas"`
	Type       int      `bson:"type" json:"type"`
	Speciality string   `bson:"speciality" json:"speciality"`
}

type meta struct {
	Type int    `bson:"metatype" json:"metatype"`
	Id   string `bson:"metaid" json:"metaid"`
}

//
// USERDATA
type userData struct {
	Id       string `bson:"_id" json:"_id"`
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

type questionToStudent struct {
	Question string
	Answer   string
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

	// // AUTH
	// koki, ok := r.GetCookie("koki")
	// if !ok || len(koki) < 5 {
	// 	return suckhttp.NewResponse(403, "Forbidden"), nil
	// }
	// //

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

	// GET QUESTIONS FROM GAK TO STUDENT
	var questionsToStudent = make([]questionToStudent, 0)

	for _, q := range quizResults {
		if question, ok := q.Answers["questionToStudentId"]; ok && len(question) > 0 {
			if answer, ok := q.Answers["answerFromStudentId"]; ok && len(answer) > 0 {
				questionsToStudent = append(questionsToStudent, questionToStudent{Question: question[0], Answer: answer[0]})
			}

		}
	}
	//

	filename := suckutils.ConcatThree("protocol", strconv.Itoa(len(questions)), ".docx")

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
	if err = getSomeJsonData(getUserDataReq, conf.getFolders, l, &studentUserData); err != nil {
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	//

	// GET FOLDER'S (VKR) DATA
	getFoldersReqVkr, err := conf.getFolders.CreateRequestFrom(suckhttp.GET, suckutils.ConcatTwo("/?folderid=", folderId), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	var vkrFolderData folder

	if err = getSomeJsonData(getFoldersReqVkr, conf.getFolders, l, &vkrFolderData); err != nil {
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	// GET NAUCHRUK'S ID
	nauchRukUserData := userData{}
	for _, metauser := range vkrFolderData.Metas {
		if metauser.Type == 5 {
			nauchRukUserData.Id = metauser.Id
		}
	}
	if nauchRukUserData.Id == "" {
		l.Error("Vkr's nauchruk", errors.New("no nauchruk"))
		return suckhttp.NewResponse(400, "Bad requiest"), nil //???????????????????????????????????????????????????????????????????????????????????????
	}

	//

	// GET GAK'S DATA
	gakUserData := make([]userData, len(quizResults))

	for i, userResult := range quizResults {
		getUserDataReq, err := conf.getUserData.CreateRequestFrom(suckhttp.GET, suckutils.ConcatThree("/", userResult.UserId, "?fields=surname,name,otch"), r)
		if err != nil {
			l.Error("CreateRequestFrom", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		if err = getSomeJsonData(getUserDataReq, conf.getFolders, l, &gakUserData[i]); err != nil {
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		if userResult.UserId == nauchRukUserData.Id {
			nauchRukUserData.Surname = gakUserData[i].Surname
			nauchRukUserData.Name = gakUserData[i].Name
			nauchRukUserData.Otch = gakUserData[i].Otch
		}
	}
	//

	// GET NAUCHRUK'S DATA IF NOT IN GAK (так может быть?)
	if nauchRukUserData.Surname == "" {
		getUserDataReq, err := conf.getUserData.CreateRequestFrom(suckhttp.GET, suckutils.ConcatThree("/", nauchRukUserData.Id, "?fields=surname,name,otch"), r)
		if err != nil {
			l.Error("CreateRequestFrom", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		if err = getSomeJsonData(getUserDataReq, conf.getFolders, l, &nauchRukUserData); err != nil {
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
	}
	//

	// GET FOLDER'S (GROUP) DATA
	getFoldersReqGroup, err := conf.getFolders.CreateRequestFrom(suckhttp.GET, suckutils.ConcatTwo("/?folderid=", folderId), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	var groupFolderData folder

	if err = getSomeJsonData(getFoldersReqGroup, conf.getFolders, l, &groupFolderData); err != nil {
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
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
	for i, q := range questions {
		edit.Replace(suckutils.ConcatThree("{question", strconv.Itoa(i), "}"), q, -1)
	}
	edit.GetContent()
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

func getSomeJsonData(req *suckhttp.Request, conn *httpservice.InnerService, l *logger.Logger, data interface{}) error {

	req.AddHeader(suckhttp.Accept, "application/json")
	resp, err := conn.Send(req)
	if err != nil {
		l.Error("Send", err)
		return err
	}

	if i, t := resp.GetStatus(); i/100 != 2 {
		l.Debug("Resp.GetStatus", t)
		return err
	}
	if len(resp.GetBody()) == 0 {
		l.Error("Resp.GetBody", errors.New("empty body at 200"))
		return err
	}

	if err := json.Unmarshal(resp.GetBody(), data); err != nil {
		l.Error("Unmarshal", err)
		return err
	}
	return nil
}
