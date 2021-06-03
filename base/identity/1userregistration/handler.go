package main

import (
	"strings"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/suckhttp"
	"github.com/tarantool/go-tarantool"
)

type UserRegistration struct {
	trntlConn  *tarantool.Connection
	trntlTable string
}

func NewUserRegistration(trntlAddr string, trntlTable string) (*UserRegistration, error) {

	trntlConnection, err := tarantool.Connect(trntlAddr, tarantool.Opts{
		// User: ,
		// Pass: ,
		Timeout:       500 * time.Millisecond,
		Reconnect:     1 * time.Second,
		MaxReconnects: 4,
	})
	if err != nil {
		return nil, err
	}
	_, err = trntlConnection.Ping()
	if err != nil {
		return nil, err
	}
	logger.Info("Tarantool", "Connected!")

	return &UserRegistration{trntlConn: trntlConnection, trntlTable: trntlTable}, nil
}

func (c *UserRegistration) Close() error {
	return c.trntlConn.Close()
}

func (conf *UserRegistration) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "text/plain") || r.GetMethod() != suckhttp.PUT {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	login := r.Uri.Path
	login = strings.Trim(login, "/")
	if login == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	password := string(r.Body)
	if password == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	if err := conf.trntlConn.UpsertAsync(conf.trntlTable, []interface{}{login, password}, []interface{}{[]interface{}{"=", "password", password}}).Err(); err != nil {
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
