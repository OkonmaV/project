package main

import (
	"strings"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/suckhttp"
	"github.com/tarantool/go-tarantool"
)

type Verify struct {
	trntlConn  *tarantool.Connection
	trntlTable string
}

func (handler *Verify) Close() error {
	return handler.trntlConn.Close()
}

func NewVerify(trntlAddr string, trntlTable string) (*Verify, error) {

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
	return &Verify{trntlConn: trntlConn, trntlTable: trntlTable}, nil
}

func (conf *Verify) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "text/plain") {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	userId := r.Uri.Query().Get("id")
	if userId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	uuid := string(r.Body)
	if uuid == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// tarantool select
	var trntlRes []interface{}
	if err := conf.trntlConn.SelectTyped(conf.trntlTable, "secondary", 0, 1, tarantool.IterEq, []interface{}{userId, uuid}, trntlRes); err != nil {
		return nil, err
	}

	if len(trntlRes) == 0 {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	//

	return suckhttp.NewResponse(200, "OK"), nil
}
