package main

import (
	"math/rand"
	"strconv"
	"strings"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/suckhttp"
	"github.com/tarantool/go-tarantool"
)

type CodeGeneration struct {
	trntlConn  *tarantool.Connection
	trntlTable string
}

func (handler *CodeGeneration) Close() error {
	return handler.trntlConn.Close()
}

func NewCodeGeneration(trntlAddr string, trntlTable string) (*CodeGeneration, error) {

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
	return &CodeGeneration{trntlConn: trntlConn, trntlTable: trntlTable}, nil
}

func (conf *CodeGeneration) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	metaId := r.Uri.Path
	metaSurname := r.Uri.Query().Get("surname")
	metaName := r.Uri.Query().Get("name")
	if metaId == "" || metaSurname == "" || metaName == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	var code int
	for {
		code = int(rnd.Int31n(90000) + 10000)
		_, err := conf.trntlConn.Insert(conf.trntlTable, []interface{}{code, "", metaId, metaSurname, metaName, "", "", 0})
		if err != nil {
			if tarErr, ok := err.(tarantool.Error); ok && tarErr.Code == tarantool.ErrTupleFound {
				continue
			} else {
				return nil, err
			}
		}
		break
	}
	resp := suckhttp.NewResponse(200, "OK")
	var body []byte
	if strings.Contains(r.GetHeader(suckhttp.Accept), "text/plain") {
		body = []byte(strconv.Itoa(code))
		resp.AddHeader(suckhttp.Content_Type, "text/plain")
	}
	resp.SetBody(body)
	return resp, nil
}
