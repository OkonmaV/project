package main

import (
	"encoding/json"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/suckhttp"
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
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	tokenString := strings.Trim(r.Uri.Path, "/")

	if tokenString == "" {
		if tokenString, _ = r.GetCookie(conf.cookieName); tokenString == "" {
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
	} else {
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
