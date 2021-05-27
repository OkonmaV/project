package main

import (
	"encoding/json"
	"strings"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/suckhttp"
	"github.com/tarantool/go-tarantool"
)

type EmailVerify struct {
	trntlConn  *tarantool.Connection
	trntlTable string
}

func (handler *EmailVerify) Close() error {
	return handler.trntlConn.Close()
}

func NewEmailVerify(trntlAddr string, trntlTable string) (*EmailVerify, error) {

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
	return &EmailVerify{trntlConn: trntlConn, trntlTable: trntlTable}, nil
}

func (conf *EmailVerify) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/json") {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	if len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	var info map[string]interface{}
	err := json.Unmarshal(r.Body, &info)
	if err != nil {
		return suckhttp.NewResponse(400, "Bad Request"), err
	}

	if info["code"] == "" || info["uuid"] == "" {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	// tarantool update
	_, err = conf.trntlConn.Update(conf.trntlTable, "secondary", []interface{}{info["code"], info["uuid"]}, []interface{}{[]interface{}{"=", "status", 2}})
	if err != nil {
		if tarErr, ok := err.(tarantool.Error); ok && tarErr.Code == tarantool.ErrTupleNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}
	//

	return suckhttp.NewResponse(200, "OK"), nil
}
