package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type Handler struct {
	tokenDecoder   *httpservice.InnerService
	getUserData    *httpservice.InnerService
	getQuizResults *httpservice.InnerService
	getFolders     *httpservice.InnerService
	template       *template.Template
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

type templateData struct {
	QuizId    string
	EntityId  string
	UserId    string
	Questions map[string]questionResult
	Datetime  time.Time
	UserData  *userData
}

type questionResult struct {
	QuestionId   string `json:"qid"`
	TextQuestion string
	TextAnswer   string
	Type         int
	Answers      map[string]answerResult
}

type answerResult struct {
	Text    string
	Checked bool
}

func NewHandler(templ *template.Template, tokendecoder, getuserdata, getquizresults, getfolders *httpservice.InnerService) (*Handler, error) {
	return &Handler{template: templ, tokenDecoder: tokendecoder, getUserData: getuserdata, getQuizResults: getquizresults, getFolders: getfolders}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	folderId := strings.Trim(r.Uri.Path, "/")
	if folderId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	quizId := r.Uri.Query().Get("quizid")
	if quizId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// // AUTH
	// koki, ok := r.GetCookie("koki")
	// if !ok || len(koki) < 5 {
	// 	return suckhttp.NewResponse(403, "Forbidden"), nil
	// }
	// //

	// Get Quiz Results For UserId & FolderId

	quizResultsReq, err := conf.getQuizResults.CreateRequestFrom(suckhttp.GET, suckutils.Concat("/", quizId, "?all=1&entityid=", folderId), r)
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

	quizResults := []*templateData{}

	if err = json.Unmarshal(quizResultsResp.GetBody(), &quizResults); err != nil {
		l.Error("UnmarshalQuizResult", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

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
	var studentUserData userData
	nauchRukUserData := userData{}
	for _, metauser := range vkrFolderData.Metas {
		if metauser.Type == 5 {
			nauchRukUserData.Id = metauser.Id
		}
		if metauser.Type == 1 {
			studentUserData.Id = metauser.Id
		}
	}
	// if nauchRukUserData.Id == "" {
	// 	l.Error("Vkr's nauchruk", errors.New("no nauchruk"))
	// 	return suckhttp.NewResponse(400, "Bad requiest"), nil //???????????????????????????????????????????????????????????????????????????????????????
	// }

	getUserDataReq, err := conf.getUserData.CreateRequestFrom(suckhttp.GET, suckutils.ConcatThree("/", studentUserData.Id, "?fields=surname,name,otch"), r) // no metaid??
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if err = getSomeJsonData(getUserDataReq, conf.getUserData, l, &studentUserData); err != nil {
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	// GET FOLDER'S (VKR) DATA

	//

	// GET GAK'S DATA
	gakUserData := make([]userData, len(quizResults))

	for i, userResult := range quizResults {
		getUserDataReq, err := conf.getUserData.CreateRequestFrom(suckhttp.GET, suckutils.ConcatThree("/", userResult.UserId, "?fields=surname,name,otch"), r)
		if err != nil {
			l.Error("CreateRequestFrom", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		if err = getSomeJsonData(getUserDataReq, conf.getUserData, l, &gakUserData[i]); err != nil {
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		userResult.UserData = &(gakUserData[i])
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
	// getFoldersReqGroup, err := conf.getFolders.CreateRequestFrom(suckhttp.GET, suckutils.ConcatTwo("/?folderid=", folderId), r)
	// if err != nil {
	// 	l.Error("CreateRequestFrom", err)
	// 	return suckhttp.NewResponse(500, "Internal Server Error"), nil
	// }
	// var groupFolderData folder

	// if err = getSomeJsonData(getFoldersReqGroup, conf.getFolders, l, &groupFolderData); err != nil {
	// 	return suckhttp.NewResponse(500, "Internal Server Error"), nil
	// }
	//

	// Generating protocol

	// doc, err := docx.ReadDocxFile("protocol.docx")
	// if err != nil {
	// 	l.Error("ReadDOCX", err)
	// 	return suckhttp.NewResponse(500, "Internal Server Error"), nil
	// }
	// edit := doc.Editable()
	// edit.Replace("{группа}", clms, -1)
	// for i, q := range questions {
	// 	edit.Replace(suckutils.ConcatThree("{question", strconv.Itoa(i), "}"), q, -1)
	// }
	// edit.GetContent()
	// doc.Close()
	// buf := &bytes.Buffer{}
	// err = edit.Write(buf)
	// if err != nil {
	// 	l.Error("WriteDOCX", err)
	// 	return suckhttp.NewResponse(500, "Internal Server Error"), nil
	// }

	// res := suckhttp.NewResponse(200, "OK")
	// res.AddHeader(suckhttp.Content_Type, mime.TypeByExtension(".docx"))
	// res.SetBody(buf.Bytes())

	var body []byte
	var contentType string
	buf := bytes.NewBuffer(body)
	err = conf.template.Execute(buf, struct {
		Results []*templateData
	}{
		Results: quizResults,
	})
	if err != nil {
		l.Error("Template execution", err)
		return suckhttp.NewResponse(500, "Internal server error"), err
	}
	body = buf.Bytes()
	contentType = "text/html"

	return suckhttp.NewResponse(200, "OK").SetBody(body).AddHeader(suckhttp.Content_Type, contentType), nil
}

func getSomeJsonData(req *suckhttp.Request, conn *httpservice.InnerService, l *logger.Logger, data interface{}) error {

	req.AddHeader(suckhttp.Accept, "application/json")
	resp, err := conn.Send(req)
	if err != nil {
		l.Error("Send", err)
		return err
	}

	if i, t := resp.GetStatus(); i/100 != 2 {
		l.Debug("Resp.GetStatus "+req.Uri.String(), t)
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
