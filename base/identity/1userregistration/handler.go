package main

import (
	"encoding/json"
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

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/json") {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	if len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	var info map[string]string
	err := json.Unmarshal(r.Body, &info)
	if err != nil {
		return suckhttp.NewResponse(400, "Bad Request"), err
	}

	if info["hash"] == "" || info["password"] == "" {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	_, err = conf.trntlConn.Insert(conf.trntlTable, []interface{}{info["hash"], info["password"]})
	if err != nil {
		if tarErr, ok := err.(tarantool.Error); ok && tarErr.Code == tarantool.ErrTupleFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
