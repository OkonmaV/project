package main

import (
	"context"
	"errors"
	"net/url"
	"project/httpservice"
	"project/test/types"
	"strings"
	"time"

	repoTarantool "project/repo/tarantool"
	repoAuthentication "project/services/authentication/repo"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/tarantool/go-tarantool"
)

//read this from configfile
type config struct {
	TrntlAddr  string
	TrntlTable string
}

//your shit here
type service struct {
	trntlConn  *tarantool.Connection
	trntlTable string
	tokenGen   *httpservice.Publisher
}

const thisServiceName httpservice.ServiceName = "authentication.login"
const pubname httpservice.ServiceName = "authentication.tokengenerator"

const expiresHours = 24

func (c *config) CreateHandler(ctx context.Context, pubs_getter httpservice.Publishers_getter) (httpservice.HTTPService, error) {
	conn, err := repoTarantool.ConnectToTarantool(c.TrntlAddr)
	if err != nil {
		return nil, err
	}

	s := &service{trntlConn: conn, trntlTable: c.TrntlTable, tokenGen: pubs_getter.Get(pubname)}

	return s, nil
}

func (s *service) Handle(r *suckhttp.Request, l types.Logger) (*suckhttp.Response, error) {
	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || r.GetMethod() != suckhttp.POST {
		l.Debug("Request", "not POST or wrong content-type")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	formValue, err := url.ParseQuery(string(r.Body))
	if err != nil {
		l.Error("Request/Parsing r.Body", err)
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	login := formValue.Get("login")
	password := formValue.Get("password")
	if len(login) == 0 || len(password) == 0 {
		l.Error("Request", errors.New("login or password not specified in body"))
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	hashLogin, err := repoAuthentication.GetMD5(login)
	if err != nil {
		l.Error("Getting md5", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	hashPassword, err := repoAuthentication.GetMD5(password)
	if err != nil {
		l.Error("Getting md5", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	var trntlAuthRec []repoAuthentication.TarantoolAuthTuple
	if err = s.trntlConn.SelectTyped(s.trntlTable, "secondary", 0, 1, tarantool.IterEq, []interface{}{hashLogin, hashPassword}, &trntlAuthRec); err != nil {
		return nil, err
	}
	if len(trntlAuthRec) == 0 {
		l.Debug("Tarantool Select", "pair login+pass not found")
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	tokenreq, err := httpservice.CreateHTTPRequestFrom(suckhttp.GET, suckutils.ConcatTwo("/", trntlAuthRec[0].UserId), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	tokenreq.AddHeader(suckhttp.Accept, "application/json")
	tokenreq.Body = []byte(hashLogin)
	tokenresp, err := s.tokenGen.SendHTTP(tokenreq)
	if err != nil {
		l.Error("Send req to tokengenerator", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if i, t := tokenresp.GetStatus(); i/100 != 2 {
		l.Error("Resp from tokengenerator", errors.New(suckutils.ConcatTwo("statuscode is ", t)))
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if len(tokenresp.GetBody()) == 0 {
		l.Error("Resp from tokengenerator", errors.New("body is empty"))
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	expires := time.Now().Add(expiresHours * time.Hour).String()
	resp := suckhttp.NewResponse(302, "Found")
	resp.SetHeader(suckhttp.Location, "/")
	resp.SetHeader(suckhttp.Set_Cookie, suckutils.Concat(repoAuthentication.CookieName, "=", string(tokenresp.GetBody()), ";Expires=", expires))

	return resp, nil
}

// may be omitted
func (s *service) Close() error {
	return s.trntlConn.Close()
}

func main() {
	httpservice.InitNewService(thisServiceName, &config{}, false, 1, pubname)
}
