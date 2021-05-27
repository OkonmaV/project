package main

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/rs/xid"
	"github.com/tarantool/go-tarantool"
)

type CreateMetauser struct {
	trntlConn      *tarantool.Connection
	trntlTable     string
	mgoSession     *mgo.Session
	mgoColl        *mgo.Collection
	codeGeneration *httpservice.InnerService
}

type metauser struct {
	MetaId  string `bson:"_id" json:"metaid"`
	Surname string `bson:"surname" json:"surname"`
	Name    string `bson:"name" json:"name"`
	Code    string `bson:"-" json:"regcode"`
}

func NewCreateMetauser(trntlAddr string, trntlTable string, mgodb string, mgoAddr string, mgoColl string, codeGeneration *httpservice.InnerService) (*CreateMetauser, error) {
	trntlConnection, err := tarantool.Connect(trntlAddr, tarantool.Opts{
		// User: ,
		// Pass: ,
		Timeout:       500 * time.Millisecond,
		Reconnect:     1 * time.Second,
		MaxReconnects: 4,
	})
	if err != nil {
		logger.Error("Tarantool", err)
		return nil, err
	}
	logger.Info("Tarantool", "Connected!")

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		logger.Error("Mongo conn", err)
		return nil, err
	}
	logger.Info("Mongo", "Connected!")
	mgoCollection := mgoSession.DB(mgodb).C(mgoColl)

	return &CreateMetauser{trntlConn: trntlConnection, trntlTable: trntlTable, mgoSession: mgoSession, mgoColl: mgoCollection, codeGeneration: codeGeneration}, nil
}

func (handler *CreateMetauser) Close() error {
	handler.mgoSession.Close()
	return handler.trntlConn.Close()
}

func getRandId() string {
	return xid.New().String()
}

func (conf *CreateMetauser) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") {
		l.Debug("Content-type", "Wrong content-type at POST")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad Request"), err
	}
	//contextFolderId = formValues.Get("contextfid")

	// TODO: AUTH

	metaSurname := formValues.Get("surname")
	metaName := formValues.Get("name")

	if metaName == "" || metaSurname == "" {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	metaId := getRandId()
	if metaId == "" {
		return nil, errors.New("returned empty string")
	}

	codeGenerationReq, err := conf.codeGeneration.CreateRequestFrom(suckhttp.POST, "", r)
	if err != nil {
		return nil, err
	}
	codeGenerationReq.Body = []byte(metaId)
	codeGenerationReq.AddHeader(suckhttp.Content_Type, "text/plain")
	codeGenerationReq.AddHeader(suckhttp.Accept, "text/plain")
	codeGenerationResp, err := conf.codeGeneration.Send(codeGenerationReq)
	if err != nil {
		return nil, err
	}

	if i, t := codeGenerationResp.GetStatus(); i != 200 {
		return nil, errors.New(suckutils.ConcatTwo("Responce from codegeneration is", t))
	}

	code := codeGenerationResp.GetBody()
	// if len(code)==0 ??

	err = conf.mgoColl.Insert(&metauser{MetaId: metaId, Surname: metaSurname, Name: metaName})
	if err != nil {
		return nil, err
	}

	resp := suckhttp.NewResponse(200, "OK")
	var body []byte
	if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {
		var err error
		body, err = json.Marshal(&metauser{MetaId: metaId, Code: string(code), Surname: metaSurname, Name: metaName})
		if err != nil {
			return resp, err // ??
		}
		resp.AddHeader(suckhttp.Content_Type, "application/json")
	}
	resp.SetBody(body)

	return resp, nil
}
