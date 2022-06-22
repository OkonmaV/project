package main

import (
	"project/base/identity/repo"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/suckhttp"
	"github.com/tarantool/go-tarantool"
)

type Handler struct {
	trntlConn  *tarantool.Connection
	trntlTable string
}

func NewHandler(trntlAddr string, trntlTable string) (*Handler, error) {

	trntlConn, err := repo.ConnectToTarantool(trntlAddr)
	if err != nil {
		return nil, err
	}
	return &Handler{trntlConn: trntlConn, trntlTable: trntlTable}, nil
}

func (handler *Handler) Close() error {
	return handler.trntlConn.Close()
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "text/plain") {
		l.Debug("Request", "not POST or wrong content-type")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	userId := strings.Trim(r.Uri.Path, "/")
	if userId == "" {
		l.Debug("Request", "userId (path) not specified")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	uuid := string(r.Body)
	if uuid == "" {
		l.Debug("Request", "uuid (body) not specified")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// tarantool select
	var trntlRes []interface{}
	if err := conf.trntlConn.SelectTyped(conf.trntlTable, "secondary", 0, 1, tarantool.IterEq, []interface{}{userId, uuid}, &trntlRes); err != nil {
		return nil, err
	}

	if len(trntlRes) == 0 {
		l.Debug("Tarantool Select", "pair userId+uuid not found")
		return suckhttp.NewResponse(404, "Not Found"), nil
	}

	//

	return suckhttp.NewResponse(200, "OK"), nil
}
