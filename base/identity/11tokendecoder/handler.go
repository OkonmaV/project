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

	if !strings.Contains(r.GetHeader(suckhttp.Accept), "application/json") {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	tokenString := r.Uri.Query().Get("token")
	if tokenString == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	res := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenString, res, func(token *jwt.Token) (interface{}, error) {
		return conf.jwtKey, nil
	})
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(&res)
	if err != nil {
		return nil, err
	}
	resp := suckhttp.NewResponse(200, "OK")
	resp.SetBody(body)

	return resp, nil

}
