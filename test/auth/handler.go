package main

import (
	"fmt"
	"thin-peak/logs/logger"

	"github.com/big-larry/suckhttp"
)

type Handler struct {
}

func NewHandler() (*Handler, error) {
	return &Handler{}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {
	//InitNewAuthorizer("", 0, 0)
	fmt.Println("SO")
	resp := suckhttp.NewResponse(200, "OK")
	return resp, nil
}
