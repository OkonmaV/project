package main

import (
	"errors"
	"mime"
	"os"
	"path"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type Handler struct {
	Path string
}

func NewHandler(path string) (*Handler, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("Path not set")
	}
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, errors.New("Path is not dir")
	}
	return &Handler{path}, nil
}

func (handler *Handler) Handle(req *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {
	filename := suckutils.ConcatTwo(handler.Path, req.Uri.Path)

	_, err := os.Stat(filename)
	if err != nil {
		l.Error("GetFile", err)
		return suckhttp.NewResponse(404, "Not found"), nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		l.Error("ReadFile", err)
		return suckhttp.NewResponse(404, "Not found"), nil
	}
	contentType := mime.TypeByExtension(path.Ext(filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	response := suckhttp.NewResponse(200, "OK")
	response.AddHeader(suckhttp.Content_Type, contentType)
	response.SetBody(data)
	return response, nil
}
