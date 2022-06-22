package main

import (
	"encoding/json"
	"project/base/quizes/repo"
	"strings"
	"text/template"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type Handler struct {
	template       *template.Template
	mgoColl        *mgo.Collection
	mgoCollResults *mgo.Collection
	auth           *httpservice.Authorizer
}

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
	Login  string `json:"login"`
	MetaId string `json:"metaid"`
	Role   int    `json:"role"`
}

func NewHandler(templ *template.Template, mgocoll *mgo.Collection, mgocollres *mgo.Collection, auth, tokendecoder *httpservice.InnerService) (*Handler, error) {

	authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{template: templ, mgoColl: mgocoll, mgoCollResults: mgocollres, auth: authorizer}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		l.Debug("Request", "not GET")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	params := make(map[string]string)

	params["quizid"] = strings.Trim(r.Uri.Path, "/")
	if _, err := bson.NewObjectIdFromHex(params["quizid"]); err != nil {
		l.Debug("Request", "quizId (path) is empty or not objectId")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	//TODO: чета хуйня тут с кукой
	var cookieClaims cookieData
	_, err := conf.auth.GetAccessWithData(r, l, "folders", 1, &cookieClaims)
	if err != nil {
		return nil, err
	}
	// if !k {
	// 	return suckhttp.NewResponse(403, "Forbidden"), nil
	// }
	// if cookieClaims.Role == 1 { // TODO HACK
	// 	return suckhttp.NewResponse(403, "Forbidden"), nil
	// }
	//

	if userId := strings.TrimSpace(r.Uri.Query().Get("userid")); userId != "" {
		if _, err = bson.NewObjectIdFromHex(params["userid"]); err != nil {
			l.Debug("Request", "specified userid isnt objectid")
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		params["userid"] = userId
	}

	if entityId := strings.TrimSpace(r.Uri.Query().Get("entityid")); entityId != "" {
		params["entityid"] = entityId
	}

	quiz, err := repo.GetQuiz(params["quizid"], conf.mgoColl)
	if err != nil {
		if err == mgo.ErrNotFound {
			l.Debug("getQuiz", "no quizes with this id founded")
			return suckhttp.NewResponse(404, "Not found"), nil // 404?
		}
		return nil, err
	}

	quizResults, err := repo.GetQuizResults(params["quizid"], params["userid"], params["entityid"], conf.mgoCollResults)
	if err != nil {
		return nil, err
	}
	if len(quizResults) == 0 {
		// TODO: ???????????? ЧТО ДЕЛАТЬ ЕСЛИ НЕТ РУЗЕЛЬТАТОВ???????????????????????????????????????????????????????????????????????????????????????
		// quizResults = make([]*results, 1)
		// quizResults[0] = &results{QuizId: params["quizid"], EntityId: params["entityid"], UserId: params["userid"]}
	}
	data := make([]templateData, len(quizResults))

	for i, res := range quizResults {
		data[i] = templateData{QuizId: res.QuizId.Hex(), EntityId: res.EntityId, UserId: res.UserId.Hex(), Datetime: res.Datetime}
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
		body, err = repo.ExecuteTemplate(conf.template, data)
		if err != nil {
			return nil, err
		}
		contentType = "text/html"
	}

	return suckhttp.NewResponse(200, "OK").SetBody(body).AddHeader(suckhttp.Content_Type, contentType), nil
}
