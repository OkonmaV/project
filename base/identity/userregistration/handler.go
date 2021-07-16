package main

import (
	"project/base/identity/repo"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/suckhttp"
	"github.com/tarantool/go-tarantool"
)

type UserRegistration struct {
	trntlConn  *tarantool.Connection
	trntlTable string
}

func NewUserRegistration(trntlAddr string, trntlTable string) (*UserRegistration, error) {

	trntlConnection, err := repo.ConnectToTarantool(trntlAddr, trntlTable)
	if err != nil {
		return nil, err
	}

	return &UserRegistration{trntlConn: trntlConnection, trntlTable: trntlTable}, nil
}

func (c *UserRegistration) Close() error {
	return c.trntlConn.Close()
}

func (conf *UserRegistration) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "text/plain") || r.GetMethod() != suckhttp.PUT {
		l.Debug("Request", "not PUT or content-type not text-plain")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	login := strings.Trim(r.Uri.Path, "/")
	if len(login) != 32 {
		l.Debug("Request", "login (path) not specified correctly")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	password := string(r.Body)
	if len(password) != 32 {
		l.Debug("Request", "password (body) not specified correctly")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	if err := conf.trntlConn.UpsertAsync(conf.trntlTable, []interface{}{login, password}, []interface{}{[]interface{}{"=", "password", password}}).Err(); err != nil {
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
