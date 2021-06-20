package main

import (
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/bradfitz/gomemcache/memcache"
)

type Handler struct {
	conn *memcache.Client
}

func NewHandler(memcs string) (*Handler, error) {
	conn := memcache.New(memcs)
	err := conn.Ping()
	if err != nil {
		return nil, err
	}
	logger.Info("Checking memcached", memcs)
	return &Handler{conn}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	id := strings.Trim(r.Uri.Path, "/")
	if id == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	if err := conf.conn.Add(&memcache.Item{Key: suckutils.ConcatTwo("broadcast.", id), Value: []byte{1}}); err != nil {
		l.Error("AddKey", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	return suckhttp.NewResponse(200, "OK"), nil
}
