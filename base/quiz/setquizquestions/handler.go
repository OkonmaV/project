package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/url"
	"strconv"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type Handler struct {
	mgoColl *mgo.Collection
	auth    *httpservice.Authorizer
}

//quiz
type quiz struct {
	Id        bson.ObjectId       `bson:"_id"`
	Name      string              `bson:"name"`
	Questions map[string]question `bson:"questions"`
	CreatorId string              `bson:"creatorid"`
}

type question struct {
	Type     int               `bson:"question_type"`
	Position int               `bson:"question_position"`
	Text     string            `bson:"question_text"`
	Answers  map[string]string `bson:"question_answers,omitempty"`
}

func NewHandler(col *mgo.Collection, auth *httpservice.InnerService, tokendecoder *httpservice.InnerService) (*Handler, error) {
	authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{mgoColl: col, auth: authorizer}, nil
}

var eq_ch []byte = []byte("=")
var amp_ch []byte = []byte("&")
var field_name []byte = []byte("data")

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST || len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	quizId, err := bson.NewObjectIdFromHex(strings.Trim(r.Uri.Path, "/"))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	var data string

	//TODO: CHECK IF QUESTIONS ALREADY EXISTS

	if strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") {
		t := bytes.Split(r.Body, amp_ch)
		for _, d := range t {
			v := bytes.SplitN(d, eq_ch, 2)
			fmt.Println("VVVVVVvvv", string(v[0]))
			if bytes.Equal(v[0], field_name) {
				if unescapedString, err := url.QueryUnescape(strings.TrimSpace(string(v[1]))); err == nil {
					data = unescapedString
					break
				}
				return suckhttp.NewResponse(400, "Bad request"), nil
			}
		}
	} else if strings.Contains(r.GetHeader(suckhttp.Content_Type), "multipart/form-data") {
		if d, err := readFromFile(r, "file"); err == nil {
			data = string(d)
		} else {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
	} else {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	if len(data) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	k, _, err := conf.auth.GetAccess(r, l, quizId.Hex(), 1)
	if err != nil {
		return nil, err
	}
	if !k {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	lines := strings.Split(strings.TrimSpace(data), "\n")
	lines = append(lines, "")

	position := 1
	questions := make(map[string]*question)
	var curquestion *question
	for _, line := range lines {
		line = strings.TrimSpace(line)
		fmt.Println("LINE", line, " LEN:", len(line))
		if line == "" { // commit current question
			questions[bson.NewObjectId().Hex()] = curquestion
			curquestion = nil
		} else if curquestion == nil { // new question
			space_position := strings.Index(line, " ")
			if space_position == -1 {
				l.Error("FormatError", errors.New(line))
				return suckhttp.NewResponse(400, "Bad request"), nil
			}
			if t, err := strconv.Atoi(line[:space_position]); err == nil {
				if t == 3 {
					curquestion = &question{Type: t, Text: strings.TrimSpace(line[space_position+1:])}
				} else {
					curquestion = &question{Type: t, Text: strings.TrimSpace(line[space_position+1:]), Answers: make(map[string]string)}
				}
				curquestion.Position = position
				position++
			} else {
				l.Error("ParseError", errors.New(line))
				return suckhttp.NewResponse(400, "Bad request"), nil
			}
		} else { // answer
			curquestion.Answers[bson.NewObjectId().Hex()] = strings.TrimSpace(line)
		}
	}
	update := bson.M{"$set": bson.M{"questions": questions}}
	if err = conf.mgoColl.UpdateId(quizId, update); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil
}

func readFromFile(req *suckhttp.Request, name string) ([]byte, error) {
	_, params, err := mime.ParseMediaType(req.GetHeader("content-type"))
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(req.Body)
	mr := multipart.NewReader(reader, params["boundary"])
	var filedata []byte
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if p.FormName() != name {
			continue
		}
		filedata, err = io.ReadAll(p)
		if err != nil {
			return nil, err
		}
	}
	if filedata == nil {
		return nil, errors.New(suckutils.ConcatThree("Field ", name, " not found"))
	}
	return filedata, nil
}
