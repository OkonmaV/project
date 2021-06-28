package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"text/template"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type Handler struct {
	template       *template.Template
	auth           *httpservice.Authorizer
	getQuiz        *httpservice.InnerService
	getQuizResults *httpservice.InnerService
}

// Quizes collection
type quizz struct {
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

//

// Results collection
type results struct {
	Id       bson.ObjectId       `bson:"_id" json:"id"`
	QuizId   string              `bson:"quizid" json:"quizid"`
	EntityId string              `bson:"entityid" json:"entityid"`
	UserId   string              `bson:"userid" json:"userid"`
	Answers  map[string][]string `bson:"answers" json:"answers"`
	Datetime time.Time           `bson:"datetime" json:"datetime"`
}

//

type templateData struct {
	QuizId    string
	EntityId  string
	UserId    string
	Questions map[string]questionResult
	Datetime  time.Time
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

type cookieData struct {
	Login  string `json:"Login"`
	MetaId string `json:"metaid"`
	Role   int    `json:"role"`
}

func NewHandler(templ *template.Template, auth, tokendecoder, getquiz *httpservice.InnerService, getquizresults *httpservice.InnerService) (*Handler, error) {

	authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{template: templ, auth: authorizer, getQuiz: getquiz, getQuizResults: getquizresults}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	params := make(map[string]string)

	params["quizid"] = strings.Trim(r.Uri.Path, "/")
	if params["quizid"] == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var cookieClaims cookieData
	_, err := conf.auth.GetAccessWithData(r, l, "folders", 1, &cookieClaims)
	if err != nil {
		return nil, err
	}
	// if !k {
	// 	return suckhttp.NewResponse(403, "Forbidden"), nil
	// }
	if cookieClaims.Role == 1 { // TODO HACK
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	params["userid"] = cookieClaims.Login
	if entityId := strings.TrimSpace(r.Uri.Query().Get("entityid")); entityId != "" {
		params["entityid"] = entityId
	}

	if len(params) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	getQuizReq, err := conf.getQuiz.CreateRequestFrom(suckhttp.GET, suckutils.Concat("/", params["quizid"]), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal server error"), nil
	}

	var quiz quizz
	if err = getSomeJsonData(getQuizReq, conf.getQuiz, &quiz); err != nil {
		l.Error("getQuiz", err)
		return suckhttp.NewResponse(500, "Internal server error"), nil
	}
	//fmt.Println("GETQUIZ||||||||-----------------------", quiz, "----------------------------|||||||||||")

	getQuizResultsReq, err := conf.getQuizResults.CreateRequestFrom(suckhttp.GET, suckutils.Concat("/", params["quizid"], "?userid=", params["userid"], "&entityid=", params["entityid"]), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal server error"), nil
	}

	var quizResults []*results
	if err = getSomeJsonData(getQuizResultsReq, conf.getQuizResults, &quizResults); err != nil {
		l.Error("getQuizResults", err)
		return suckhttp.NewResponse(500, "Internal server error"), nil
	}
	//fmt.Println("|||||||||-----------------------", quizResults, "----------------------------||||||||")
	if len(quizResults) == 0 {
		quizResults = make([]*results, 1)
		quizResults[0] = &results{QuizId: params["quizid"], EntityId: params["entityid"], UserId: params["userid"]}
	}
	data := make([]templateData, len(quizResults))

	for i, res := range quizResults {
		//res.Datetime.IsZero()
		data[i] = templateData{QuizId: res.QuizId, EntityId: res.EntityId, UserId: res.UserId, Datetime: res.Datetime}
		data[i].Questions = make(map[string]questionResult)

		for questionId, question := range quiz.Questions {
			var textanswer string
			answers := make(map[string]answerResult)

			//data[i].questions[questionId] = questionResult{QuestionId: questionId, TextQuestion: question.Text, Type: question.Type}

			if question.Type == 3 && len(res.Answers[questionId]) != 0 {
				//data[i].questions[questionId].TextAnswer = res.Answers[questionId][0]
				textanswer = res.Answers[questionId][0]
			} else {
				//data[i].questions[questionId].Answers = make(map[string]answerResult)

				for ansId, ansText := range quiz.Questions[questionId].Answers {
					var checked bool
					for _, resansId := range res.Answers[questionId] {
						if resansId == ansId {
							checked = true
							break
						}
					}
					//data[i].questions[questionId].Answers[ansId] = answerResult{Text: ansText, Checked: checked}
					answers[ansId] = answerResult{Text: ansText, Checked: checked}
				}
			}
			data[i].Questions[questionId] = questionResult{QuestionId: questionId, TextQuestion: question.Text, TextAnswer: textanswer, Type: question.Type, Answers: answers}
		}

	}
	// fmt.Println("DATA|||||||||-----------------------", data, "----------------------------||||||||")
	// for i, res := range quizResults {
	// 	data[i].Results = make([]templQuestion, len(res.Answers))

	// 	for questionId, answerIds := range res.Answers {

	// 		if q, ok := quiz.Questions[questionId]; ok {
	// 			data[i].Results[cnt] = templQuestion{QuestionId: questionId, Text: q.Text}
	// 		} else {
	// 			l.Error("Quizes", errors.New(suckutils.ConcatTwo("cant find quii.question in mongo.quizes with id:", questionId)))
	// 			return suckhttp.NewResponse(500, "Internal server error"), nil
	// 		}
	// 		for i, answerId := range answerIds {

	// 		}
	// 	}
	// 	data[i].Results = append(data[i].Results, templQuestion{})
	// }

	var body []byte
	var contentType string

	if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {
		var err error
		body, err = json.Marshal(data)
		if err != nil {
			l.Error("Marshalling mongo responce", err)
			return suckhttp.NewResponse(500, "Internal server error"), nil
		}
		contentType = "application/json"

	} else if strings.Contains(r.GetHeader(suckhttp.Accept), "text/html") {
		buf := bytes.NewBuffer(body)
		err := conf.template.Execute(buf, data)
		if err != nil {
			l.Error("Template execution", err)
			return suckhttp.NewResponse(500, "Internal server error"), err
		}
		body = buf.Bytes()
		contentType = "text/html"
	}

	return suckhttp.NewResponse(200, "OK").SetBody(body).AddHeader(suckhttp.Content_Type, contentType), nil
}

func getSomeJsonData(req *suckhttp.Request, conn *httpservice.InnerService, data interface{}) error {

	req.AddHeader(suckhttp.Accept, "application/json")
	resp, err := conn.Send(req)
	if err != nil {
		return errors.New(suckutils.ConcatTwo("send: ", err.Error()))
	}

	if i, t := resp.GetStatus(); i/100 != 2 {
		return errors.New(suckutils.ConcatTwo("status: ", t))
	}
	if len(resp.GetBody()) == 0 {
		return errors.New("body: is empty")
	}

	if err := json.Unmarshal(resp.GetBody(), data); err != nil {
		return errors.New(suckutils.ConcatTwo("unmarshal: ", err.Error()))
	}
	return nil
}
