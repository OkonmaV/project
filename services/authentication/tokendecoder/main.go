package main

import (
	"context"
	"encoding/json"
	"errors"
	"project/httpservice"
	repoAuthentication "project/services/authentication/repo"
	"project/test/types"
	"strings"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/dgrijalva/jwt-go"
)

//read this from configfile
type config struct {
}

//your shit here
type service struct {
}

const thisServiceName httpservice.ServiceName = "authentication.tokendecoder"

func (c *config) CreateHandler(ctx context.Context, pubs_getter httpservice.Publishers_getter) (httpservice.HTTPService, error) {
	s := &service{}

	return s, nil
}

func (s *service) Handle(r *suckhttp.Request, l types.Logger) (*suckhttp.Response, error) {
	if r.GetMethod() != suckhttp.GET {
		l.Error("Request", errors.New("not GET"))
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	tokenString := string(r.Body)

	if tokenString == "" {
		if tokenString, _ = r.GetCookie(repoAuthentication.CookieName); len(tokenString) == 0 {
			l.Debug("Request", "empty body (expected tokenstring)")
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
	}

	cookieData := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenString, cookieData, func(token *jwt.Token) (interface{}, error) {
		return repoAuthentication.JwtKey, nil
	})
	if err != nil {
		l.Error("ParseWithClaims", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	resp := suckhttp.NewResponse(200, "OK")
	var body []byte
	var contentType string
	accept := r.GetHeader(suckhttp.Accept)
	if strings.Contains(accept, "application/json") {
		body, err = json.Marshal(cookieData)
		if err != nil {
			l.Error("Marshalling decoded data", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		contentType = "application/json"
	} else if strings.Contains(accept, "text/plain") {
		if login, ok := cookieData["login"]; ok {
			body = []byte(login.(string))
		} else {
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		contentType = "text/plain; charset=utf-8"
	} else if strings.Contains(accept, "text/html") {

		return suckhttp.NewResponse(406, "Not Acceptable"), nil
		// var surname, name interface{}
		// var ok bool
		// if surname, ok = cookieData["surname"]; !ok {
		// 	return suckhttp.NewResponse(500, "Internal Server Error"), nil
		// }
		// if name, ok = cookieData["name"]; !ok {
		// 	return suckhttp.NewResponse(500, "Internal Server Error"), nil
		// }
		// body = []byte(suckutils.Concat(`<ul class="navbar-nav mb-2 mb-sm-0"><li class="nav-item"><a class="nav-link disabled" aria-disabled="true">`, surname.(string), " ", name.(string), `</a></li><li class="nav-item"><a class="nav-link" href="/signout">Выйти</a></li></ul>`))
		// contentType = "text/html; charset=utf-8"
	} else {
		l.Error("Accept", errors.New(suckutils.ConcatTwo("unsupported accept: ", accept)))
		return suckhttp.NewResponse(406, "Not Acceptable"), nil
	}

	resp.SetBody(body)
	resp.AddHeader(suckhttp.Content_Type, contentType)

	return resp, nil

}

// may be omitted
func (s *service) Close() error {
	return nil
}

func main() {
	httpservice.InitNewService(thisServiceName, &config{}, false, 1)
}
