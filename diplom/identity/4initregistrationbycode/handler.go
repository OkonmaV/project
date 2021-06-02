package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
	"thin-peak/logs/logger"
	"time"

	"thin-peak/httpservice"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/tarantool/go-tarantool"
)

var emailRegex = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

type userData struct {
	Mail     string `json:"mail"`
	Name     string `json:"name"`
	Surname  string `json:"surname"`
	Otch     string `json:"otch"`
	Position string `json:"position"`
	Password string `json:"password"`
	//MetaId   string `json:"metaid,omitempty"`
}
type tuple struct {
	Code    int
	Data    string
	MetaId  string
	Surname string
	Name    string
	Hash    string
	Status  int
}
type InitRegistrationByCode struct {
	trntlConn         *tarantool.Connection
	trntlTable        string
	createVerifyEmail *httpservice.InnerService
}

func (handler *InitRegistrationByCode) Close() error {
	return handler.trntlConn.Close()
}

func NewInitRegistrationByCode(trntlAddr string, trntlTable string, createVerifyEmail *httpservice.InnerService) (*InitRegistrationByCode, error) {

	trntlConn, err := tarantool.Connect(trntlAddr, tarantool.Opts{
		// User: ,
		// Pass: ,
		Timeout:       500 * time.Millisecond,
		Reconnect:     1 * time.Second,
		MaxReconnects: 4,
	})
	if err != nil {
		return nil, err
	}
	logger.Info("Tarantool", "Connected!")
	return &InitRegistrationByCode{trntlConn: trntlConn, trntlTable: trntlTable, createVerifyEmail: createVerifyEmail}, nil
}

func (conf *InitRegistrationByCode) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || r.GetMethod() != suckhttp.POST {
		l.Debug("Content-type", "Wrong content-type at POST")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		l.Error("Parsing r.Body", err)
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	userCode := formValues.Get("code")
	if userCode == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	userF := formValues.Get("f")
	userI := formValues.Get("i")
	userO := formValues.Get("o")

	if len(userF) < 2 || len(userI) < 5 || len(userO) < 5 || len(userF) > 30 || len(userI) > 30 || len(userO) > 30 {
		return suckhttp.NewResponse(400, "Bad Request"), nil // TODO: bad request ли?
	}

	userPassword := formValues.Get("password")
	if len(userPassword) < 8 || len(userPassword) > 30 {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}
	userMail := formValues.Get("mail")
	if !isEmailValid(userMail) {
		return suckhttp.NewResponse(400, "Bad Request"), nil // TODO: bad request ли?
	}
	userMailHashed, err := getMD5(userMail)
	if err != nil {
		l.Error("Getting md5", err)
		return nil, nil
	}

	userPasswordHashed, err := getMD5(userPassword)
	if err != nil {
		l.Error("Getting md5", err)
		return nil, nil
	}

	// tarantool update
	userDataMarshalled, err := json.Marshal(&userData{Mail: userMail, Name: userI, Surname: userF, Otch: userO})
	if err != nil {
		l.Error("Marshalling userData", err)
		return nil, nil
	}

	update := []interface{}{[]interface{}{"=", "data", string(userDataMarshalled)}, []interface{}{"+", "status", 1}, []interface{}{"=", "hash", userMailHashed}, []interface{}{"=", "password", userPasswordHashed}}
	var trntlRes []tuple
	if err = conf.trntlConn.UpdateAsync(conf.trntlTable, "primary", []interface{}{userCode}, update).GetTyped(&trntlRes); err != nil {
		return nil, err // добавить поле статус и перенести апдейт в начало . проверка на дудос!!! второй операцией
	}

	if len(trntlRes) == 0 {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	if trntlRes[0].Status > 5 {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	//

	// createVerifyEmail request
	createVerifyEmailReq, err := conf.createVerifyEmail.CreateRequestFrom(suckhttp.PUT, userMailHashed, r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return nil, nil
	}
	createVerifyEmailReq.AddHeader(suckhttp.Accept, "text/plain")
	createVerifyEmailResp, err := conf.createVerifyEmail.Send(createVerifyEmailReq)
	if err != nil {
		l.Error("Send req to createverifyemailreq", err)
		return nil, nil
	}
	if i, t := createVerifyEmailResp.GetStatus(); i != 200 {
		l.Error("Resp from createverifyemailreq", errors.New(suckutils.ConcatTwo("statuscode is ", t)))
		return nil, nil
	}

	uuid := string(createVerifyEmailResp.GetBody())
	if uuid == "" {
		l.Error("Resp from createverifyemailreq", errors.New("body is empty"))
		return nil, nil
	}
	//

	// createEmailMessage request
	//var smth //TODOется
	//

	return suckhttp.NewResponse(200, "OK"), nil
}

func isEmailValid(email string) bool {
	if len(email) < 6 && len(email) > 40 {
		return false
	}
	if !emailRegex.MatchString(email) {
		return false
	}
	parts := strings.Split(email, "@")
	mx, err := net.LookupMX(parts[1])
	if err != nil || len(mx) == 0 {
		return false
	}
	return true
}

func getMD5(str string) (string, error) {
	hash := md5.New()
	_, err := hash.Write([]byte(str))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
