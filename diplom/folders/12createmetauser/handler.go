package main

import (
	"encoding/json"
	"errors"
	"fmt"
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
	mgoColl        *mgo.Collection
	codeGeneration *httpservice.InnerService
	auth           *httpservice.Authorizer
}

type metauser struct {
	MetaId  string `bson:"_id" json:"metaid"`
	Surname string `bson:"surname" json:"surname"`
	Name    string `bson:"name" json:"name"`
	Code    string `bson:"-" json:"regcode"`
}

func NewHandler(mgoColl *mgo.Collection, codeGeneration *httpservice.InnerService, auth *httpservice.InnerService, tokendecoder *httpservice.InnerService) (*Handler, error) {
	// authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	// if err != nil {
	// 	return nil, err
	// }

	return &Handler{mgoColl: mgoColl, codeGeneration: codeGeneration}, nil
}

func getRandId() string {
	return bson.NewObjectId().Hex()
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || r.GetMethod() != suckhttp.POST {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		l.Error("Parsing r.Body", err)
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}
	//contextFolderId = formValues.Get("contextfid")

	// k, _, err := conf.auth.GetAccess(r, l, "createmetauser", 1)
	// if err != nil {
	// 	return nil, err
	// }
	// if !k {
	// 	return suckhttp.NewResponse(403, "Forbidden"), nil
	// }

	userRole := formValues.Get("role")
	_, err = strconv.Atoi(formValues.Get("role"))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	metaSurname := formValues.Get("surname")
	metaName := formValues.Get("name")

	if metaName == "" || metaSurname == "" {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	metaId := getRandId()
	if metaId == "" {
		l.Error("Generating uid", errors.New("returned empty string"))
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	codeGenerationReq, err := conf.codeGeneration.CreateRequestFrom(suckhttp.POST, suckutils.Concat("/", metaId, "?surname=", metaSurname, "&name=", metaName, "&role=", userRole), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	codeGenerationReq.AddHeader(suckhttp.Accept, "text/plain")
	codeGenerationResp, err := conf.codeGeneration.Send(codeGenerationReq)
	if err != nil {
		l.Error("Send req to codegeneration", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	if codeGenerationResp == nil {
		l.Info("nil", "resp")
	}

	if i, t := codeGenerationResp.GetStatus(); i != 200 {
		fmt.Println(metaId, metaSurname, metaName)
		l.Error("Resp from codegeneration", errors.New(suckutils.ConcatTwo("Responce from codegeneration is", t)))
		return codeGenerationResp, nil
	}

	code := codeGenerationResp.GetBody()
	if len(codeGenerationResp.GetBody()) == 0 {
		l.Error("Resp from codegeneration", errors.New("body is empty"))
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	if err = conf.mgoColl.Insert(&metauser{MetaId: metaId, Surname: metaSurname, Name: metaName}); err != nil {
		//TODO: err when founded?
		return nil, err
	}

	resp := suckhttp.NewResponse(200, "OK")
	var body []byte
	if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {
		var err error
		body, err = json.Marshal(&metauser{MetaId: metaId, Code: string(code), Surname: metaSurname, Name: metaName})
		if err != nil {
			l.Error("Marshalling inserted data", err)
			return resp, nil // ??
		}
		resp.AddHeader(suckhttp.Content_Type, "application/json")
	}
	resp.SetBody(body)

	return resp, nil
}
