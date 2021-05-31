package main

import (
	"thin-peak/logs/logger"

	"github.com/big-larry/suckhttp"
	"github.com/dgrijalva/jwt-go"
)

type claims struct {
	Login string
	jwt.StandardClaims
}

type CookieTokenGenerator struct {
	jwtKey []byte
}

func NewCookieTokenGenerator(jwtKey string) (*CookieTokenGenerator, error) {
	return &CookieTokenGenerator{jwtKey: []byte(jwtKey)}, nil
}

func (conf *CookieTokenGenerator) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	// NO AUTH?
..//GET
	var jwtToken string

	hashLogin := r.Uri.Query().Get("hash")

	if len(hashLogin) != 32 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	jwtToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims{Login: hashLogin}).SignedString(conf.jwtKey)
	if err != nil {
		l.Error("Generating new jwtToken", err)
		return nil, nil
	}

	resp := suckhttp.NewResponse(200, "OK")
	resp.SetBody([]byte(jwtToken))

	return resp, nil
}
