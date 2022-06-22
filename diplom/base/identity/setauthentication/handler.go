package main

import (
	"project/base/identity/repo"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/tarantool/go-tarantool"
)

type Handler struct {
	trntlConn  *tarantool.Connection
	trntlTable string
}

func NewHandler(trntlAddr string, trntlTable string) (*Handler, error) {
	trntlConnection, err := repo.ConnectToTarantool(trntlAddr)
	if err != nil {
		return nil, err
	}

	return &Handler{trntlConn: trntlConnection, trntlTable: trntlTable}, nil
}

func (c *Handler) Close() error {
	return c.trntlConn.Close()
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "text/plain") || r.GetMethod() != suckhttp.PUT {
		l.Debug("Request", "method or content-type not allowed")
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

	userId, err := bson.NewObjectIdFromHex(r.Uri.Query().Get("userid"))
	if err != nil {
		l.Debug("Request", "userid (query) not correctly specified")
		return suckhttp.NewResponse(400, "Bad request"), nil

	}

	if err := conf.trntlConn.UpsertAsync(conf.trntlTable, []interface{}{login, password}, []interface{}{[]interface{}{"=", "password", password}, []interface{}{"=", "userid", userId.Hex()}}).Err(); err != nil {
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
