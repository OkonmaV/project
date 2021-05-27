package main

import (
	"strings"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/suckhttp"
	uuid "github.com/satori/go.uuid"
	"github.com/tarantool/go-tarantool"
)

type CreateVerifyEmail struct {
	trntlConn  *tarantool.Connection
	trntlTable string
}

func (handler *CreateVerifyEmail) Close() error {
	return handler.trntlConn.Close()
}

func NewCreateVerifyEmail(trntlAddr string, trntlTable string) (*CreateVerifyEmail, error) {

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
	return &CreateVerifyEmail{trntlConn: trntlConn, trntlTable: trntlTable}, nil
}

func (conf *CreateVerifyEmail) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "text/plain") {
		l.Debug("Content-type", "Wrong content-type at POST")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	code := string(r.Body)
	if code == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	uuid, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	_, err = conf.trntlConn.Insert(conf.trntlTable, []interface{}{code, uuid, 0})
	if err != nil {
		if tarErr, ok := err.(tarantool.Error); ok && tarErr.Code == tarantool.ErrTupleFound {
			return suckhttp.NewResponse(403, "Forbidden"), err
		}
		return nil, err
	}
	return suckhttp.NewResponse(200, "OK"), nil
}
