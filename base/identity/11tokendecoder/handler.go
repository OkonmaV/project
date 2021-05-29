package main

import (
	"encoding/json"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/suckhttp"
	"github.com/dgrijalva/jwt-go"
)

type TokenDecoder struct {
	jwtKey []byte
}

func NewTokenDecoder(jwtKey string) (*TokenDecoder, error) {
	return &TokenDecoder{jwtKey: []byte(jwtKey)}, nil
}

func (conf *TokenDecoder) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	// AUTH

	tokenString := r.Uri.Query().Get("token")
	if tokenString == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	res := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenString, res, func(token *jwt.Token) (interface{}, error) {
		return conf.jwtKey, nil
	})
	if err != nil {
		l.Error("Parsing token string", err)
		return nil, nil
	}

	resp := suckhttp.NewResponse(200, "OK")
	var body []byte
	var contentType string
	if strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {
		body, err = json.Marshal(res)
		if err != nil {
			l.Error("Marshalling decoded data", err)
			return nil, nil
		}
		contentType = "application/json"
	}

	resp.SetBody(body)
	resp.AddHeader(suckhttp.Content_Type, contentType)

	return resp, nil

}
