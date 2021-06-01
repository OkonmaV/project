package main

import (
	"strings"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/suckhttp"
	uuid "github.com/satori/go.uuid"
	"github.com/tarantool/go-tarantool"
)

type CreateVerify struct {
	trntlConn  *tarantool.Connection
	trntlTable string
}

func (handler *CreateVerify) Close() error {
	return handler.trntlConn.Close()
}

func NewCreateVerify(trntlAddr string, trntlTable string) (*CreateVerify, error) {

	trntlConn, err := tarantool.Connect(trntlAddr, tarantool.Opts{
		// User: ,
		// Pass: ,
		Timeout:       500 * time.Millisecond,
		Reconnect:     1 * time.Second,
		MaxReconnects: 4,
	})
	if err != nil {
		return nil, err
	}
	logger.Info("Tarantool", "Connected!")
	return &CreateVerify{trntlConn: trntlConn, trntlTable: trntlTable}, nil
}

func (conf *CreateVerify) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "text/plain") || r.GetMethod() != suckhttp.PUT { //..PUT
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	userId := string(r.Body)
	if userId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	uuid, err := uuid.NewV4()
	if err != nil {
		return nil, err // err??
	}

	if err = conf.trntlConn.UpsertAsync(conf.trntlTable, []interface{}{userId, uuid.String()}, []interface{}{[]interface{}{"=", "uuid", uuid.String()}}).Err(); err != nil {
		return nil, err
	}

	resp := suckhttp.NewResponse(200, "OK")
	var body []byte
	var contentType string
	if strings.Contains(r.GetHeader(suckhttp.Accept), "text/plain") {
		body = uuid.Bytes()
		contentType = "text/plain"
	}

	resp.SetBody(body)
	resp.AddHeader(suckhttp.Content_Type, contentType)

	return resp, nil
}
