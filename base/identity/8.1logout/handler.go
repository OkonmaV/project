package main

import (
	"thin-peak/logs/logger"

	"github.com/big-larry/suckhttp"
)

type Handler struct {
}

func NewHandler() (*Handler, error) {
	return &Handler{}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if _, ok := r.GetCookie("koki"); !ok {
		l.Debug("Get cookie", "no cookie named \"koki\" founded in request")
		return suckhttp.NewResponse(200, "OK"), nil
	}

	resp := suckhttp.NewResponse(200, "OK")
	resp.SetHeader(suckhttp.Set_Cookie, "koki=; Max-Age=-1")
	return resp, nil
}
