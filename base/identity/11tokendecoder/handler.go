package main

import (
	"encoding/json"

	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/dgrijalva/jwt-go"
)

type TokenDecoder struct {
	jwtKey     []byte
	cookieName string
}

func NewTokenDecoder(jwtKey, cookieName string) (*TokenDecoder, error) {
	return &TokenDecoder{jwtKey: []byte(jwtKey), cookieName: cookieName}, nil
}

func (conf *TokenDecoder) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {
	if r.GetMethod() != suckhttp.GET {
		l.Debug("Method", "wrong")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	tokenString := strings.Trim(r.Uri.Path, "/")

	if tokenString == "" {
		if tokenString, _ = r.GetCookie(conf.cookieName); tokenString == "" {
			l.Debug("TokenString", "empty")
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
	}

	res := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenString, res, func(token *jwt.Token) (interface{}, error) {
		return conf.jwtKey, nil
	})
	if err != nil {
		l.Error("Parsing token string", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	resp := suckhttp.NewResponse(200, "OK")
	var body []byte
	var contentType string
	if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {
		body, err = json.Marshal(res)
		if err != nil {
			l.Error("Marshalling decoded data", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		contentType = "application/json"
	} else if strings.Contains(r.GetHeader(suckhttp.Accept), "text/plain") {
		if login, ok := res["Login"]; ok {
			body = []byte(login.(string))
		} else {
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		contentType = "text/plain; charset=utf-8"
	} else if strings.Contains(r.GetHeader(suckhttp.Accept), "text/html") {
		var surname, name interface{}
		var ok bool
		if surname, ok = res["surname"]; !ok {
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		if name, ok = res["name"]; !ok {
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		body = []byte(suckutils.Concat(`<ul class="navbar-nav mb-2 mb-sm-0"><li class="nav-item"><a class="nav-link disabled" aria-disabled="true">`, surname.(string), " ", name.(string), `</a></li><li class="nav-item"><a class="nav-link" href="/signout">Выйти</a></li></ul>`))
		contentType = "text/html; charset=utf-8"
	} else {
		l.Debug("Accept", "not allowed")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	if login, ok := res["Login"]; ok {
		resp.AddHeader("userhash", login.(string))
	} else {
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	resp.SetBody(body)
	resp.AddHeader(suckhttp.Content_Type, contentType)

	return resp, nil

}
