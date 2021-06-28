package main

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type Handler struct {
	Path string
	auth *httpservice.Authorizer
}

func NewHandler(path string, auth *httpservice.Authorizer) (*Handler, error) {
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
	return &Handler{path, auth}, nil
}

func (handler *Handler) Handle(req *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {
	if req.GetMethod() != suckhttp.POST || !strings.Contains(req.GetHeader(suckhttp.Content_Type), "multipart/form-data") || len(req.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	path := suckutils.ConcatTwo(handler.Path, req.Uri.Path)

	_, _, err := handler.auth.GetAccess(req, l, "createquiz", 1)
	if err != nil {
		l.Error("Auth", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	// if !access {
	// 	return suckhttp.NewResponse(403, "Forbidden"), nil
	// }

	if _, err = os.Stat(path); os.IsNotExist(err) {
		err = os.Mkdir(path, 0755)
		if err != nil {
			l.Error("Mkdir", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
	}

	_, params, err := mime.ParseMediaType(req.GetHeader("content-type"))
	reader := bytes.NewReader(req.Body)
	mr := multipart.NewReader(reader, params["boundary"])
	filename := ""
	filefilename := ""
	var filedata []byte
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		l.Debug("Upload", p.FileName())
		if err != nil {
			l.Error("readRequest", err)
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		slurp, err := io.ReadAll(p)
		if err != nil {
			l.Error("readRequest", err)
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		// if p.FormName() == "name" {
		// 	filename = string(slurp)
		// }
		if p.FormName() == "file" {
			filedata = slurp
			filefilename = p.FileName()

			if filename == "" {
				filename = filefilename
			}
			if filename == "" {
				return suckhttp.NewResponse(400, "Bad request"), nil
			}

			err = os.WriteFile(suckutils.ConcatThree(path, "/", filename), filedata, 0644)
			if err != nil {
				l.Error("WriteFile", err)
				return suckhttp.NewResponse(500, "Internal Server Error"), nil
			}
			filename = ""
		}
	}

	return suckhttp.NewResponse(302, "Found").AddHeader(suckhttp.Location, suckutils.ConcatTwo("/edit/", req.Uri.Path)), nil
	return suckhttp.NewResponse(201, "Created"), nil
}
