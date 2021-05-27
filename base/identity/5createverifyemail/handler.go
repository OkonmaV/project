package main

import (
	"crypto/md5"
	"encoding/hex"
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

	mail := string(r.Body)
	if mail == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	uuid, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	mailHashed, err := getMD5(mail)
	if err != nil {
		return nil, err
	}

	_, err = conf.trntlConn.Insert(conf.trntlTable, []interface{}{mailHashed, uuid.String(), 0})
	if err != nil {
		if tarErr, ok := err.(tarantool.Error); ok && tarErr.Code == tarantool.ErrTupleFound {
			return suckhttp.NewResponse(403, "Forbidden"), err
		}
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

func getMD5(str string) (string, error) {
	hash := md5.New()
	_, err := hash.Write([]byte(str))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
