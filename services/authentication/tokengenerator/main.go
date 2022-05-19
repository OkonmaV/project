package main

import (
	"context"
	"encoding/json"
	"errors"
	"project/httpservice"
	repoAuthentication "project/services/authentication/repo"
	"project/test/types"
	"strings"

	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/dgrijalva/jwt-go"
)

//read this from configfile
type config struct {
}

//your shit here
type service struct {
	getUserData *httpservice.Publisher
}

const thisServiceName httpservice.ServiceName = "authentication.tokengenerator"
const publishername httpservice.ServiceName = "accounting.get"

func (c *config) CreateHandler(ctx context.Context, pubs_getter httpservice.Publishers_getter) (httpservice.HTTPService, error) {
	s := &service{}

	return s, nil
}

func (s *service) Handle(r *suckhttp.Request, l types.Logger) (*suckhttp.Response, error) {
	if r.GetMethod() != suckhttp.GET {
		l.Error("Request", errors.New("not GET"))
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	userId, err := bson.NewObjectIdFromHex(strings.Trim(r.Uri.Path, "/"))
	if err != nil {
		l.Debug("Request", suckutils.ConcatTwo("userId not correctly specified in path, err: ", err.Error()))
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	hashedLogin := string(r.Body)
	if len(hashedLogin) != 32 {
		l.Debug("Request", "hashed login not correctly specified in body (len not 32)")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	getUserDataReq, err := httpservice.CreateHTTPRequestFrom(suckhttp.GET, suckutils.ConcatThree("/", userId.Hex(), "?fields=role,surname,name"), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	getUserDataReq.AddHeader(suckhttp.Accept, "application/json")
	getUserDataResp, err := s.getUserData.SendHTTP(getUserDataReq)
	if err != nil {
		l.Error("Send to getUserData", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	if i, t := getUserDataResp.GetStatus(); i != 200 {
		l.Error("Response from getUserData", errors.New(suckutils.ConcatTwo("returned status: ", t)))
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if len(getUserDataResp.GetBody()) == 0 {
		l.Error("Response from getUserData", errors.New("empty body at 200"))
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	var clms repoAuthentication.CookieClaims
	if err := json.Unmarshal(getUserDataResp.GetBody(), &clms); err != nil {
		l.Error("Unmarshal", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	clms.Login = hashedLogin

	jwtToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &clms).SignedString(repoAuthentication.JwtKey)
	if err != nil {
		l.Error("Generating new jwtToken", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	return suckhttp.NewResponse(200, "OK").SetBody([]byte(jwtToken)), nil
}

// may be omitted
func (s *service) Close() error {
	return nil
}

func main() {
	httpservice.InitNewService(thisServiceName, &config{}, false, 1, publishername)
}
