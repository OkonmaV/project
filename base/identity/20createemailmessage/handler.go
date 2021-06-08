package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"modules/suckutils"
	"strconv"
	"strings"
	"thin-peak/logs/logger"

	"thin-peak/httpservice"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckmail"
)

type Handler struct {
	sender *httpservice.InnerService
	tmpl   *template.Template
}

type requestdata struct {
	Email      string `json:"email"`
	Hash       string `json:"hash"`
	VerifyCode string `json:"verifycode"`
	Name       string `json:"name"`
}

func NewHandler(sender *httpservice.InnerService) (*Handler, error) {
	tmpl, err := template.ParseFiles("message.html")
	if err != nil {
		return nil, err
	}
	return &Handler{sender, tmpl}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/json") || r.GetMethod() != suckhttp.POST {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var res requestdata
	if err := json.Unmarshal(r.Body, &res); err != nil {
		l.Error("Unmarshal", err)
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// TODO Verify data
	if res.Email == "" {
	}
	if res.Hash == "" {
	}
	if res.Name == "" {
	}
	if res.VerifyCode == "" {
	}

	html := &bytes.Buffer{}

	if err := conf.tmpl.Execute(html, &res); err != nil {
		l.Error("Send", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	msg := suckmail.NewMessage().
		SetHTML(html.String(), true).
		SetSubject("Регистрация").
		SetReciever(res.Email, "")

	msgdata, err := msg.Build()
	if err != nil {
		l.Error("Build", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	req, _ := suckhttp.NewRequest(suckhttp.POST, "/")
	req.AddHeader("x-request-id", r.GetHeader("x-request-id"))
	req.Body = msgdata

	resp, err := conf.sender.Send(req)
	if err != nil {
		l.Error("Send", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if code, _ := resp.GetStatus(); code/100 != 2 {
		l.Error("Send", errors.New(suckutils.ConcatTwo("Status code is ", strconv.Itoa(code))))
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	return suckhttp.NewResponse(202, "Accepted"), nil
}
