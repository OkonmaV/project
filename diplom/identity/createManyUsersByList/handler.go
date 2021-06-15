package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/url"
	"strconv"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type Handler struct {
	tokenDecoder             *httpservice.InnerService
	auth                     *httpservice.Authorizer
	userRegistration         *httpservice.InnerService
	setUserData              *httpservice.InnerService
	createOnlyMetauser       *httpservice.InnerService
	createFolderWithMetauser *httpservice.InnerService
}

type userdata struct {
	Name     string `json:"name"`
	Surname  string `json:"surname"`
	Otch     string `json:"otch"`
	Password string `json:"-"`
	MetaId   string `json:"metaid"`
	FolderId string `json:"folderid"`
	Role     int    `json:"role"`
	Group    string `json:"group"`
}

var eq_ch []byte = []byte("=")
var amp_ch []byte = []byte("&")
var field_name []byte = []byte("data")
var group_field_name []byte = []byte("group")

func NewHandler(tokendecoder *httpservice.InnerService, authGet *httpservice.InnerService, userReg *httpservice.InnerService, setUserdata *httpservice.InnerService, createMeta *httpservice.InnerService, createFolderWithMeta *httpservice.InnerService) (*Handler, error) {
	authorizer, err := httpservice.NewAuthorizer(thisServiceName, authGet, tokendecoder)
	if err != nil {
		return nil, err
	}

	return &Handler{tokenDecoder: tokendecoder, auth: authorizer, userRegistration: userReg, setUserData: setUserdata, createOnlyMetauser: createMeta, createFolderWithMetauser: createFolderWithMeta}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST || len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	usersType, err := strconv.Atoi(r.Uri.Query().Get("type"))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	// AUTH
	k, foo, err := conf.auth.GetAccess(r, l, "createmanyusers", 1)
	if err != nil {
		return nil, err
	}
	l.Info("LOGIN", foo)
	if !k {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	//

	// parse usersdata
	var data string
	var group string

	if strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") {
		t := bytes.Split(r.Body, amp_ch)
		for _, d := range t {
			v := bytes.SplitN(d, eq_ch, 2)
			if bytes.Equal(v[0], field_name) {
				unescapedString, err := url.QueryUnescape(strings.TrimSpace(string(v[1])))
				if err == nil {
					data = unescapedString
					if group != "" {
						break
					}
				} else {
					return suckhttp.NewResponse(400, "Bad request"), nil
				}
			} else if bytes.Equal(v[0], group_field_name) {
				unescapedString, err := url.QueryUnescape(strings.TrimSpace(string(v[1])))
				if err == nil {
					group = unescapedString
					if data != "" {
						break
					}
				} else {
					return suckhttp.NewResponse(400, "Bad request"), nil
				}
			}

		}
	} else if strings.Contains(r.GetHeader(suckhttp.Content_Type), "multipart/form-data") {
		if d, err := readFromFile(r, "file"); err == nil {
			data = string(d) //TODO: get group from file
		} else {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
	} else {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	if len(data) == 0 {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	lines := strings.Split(strings.TrimSpace(data), "\n")

	var users []*userdata

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue

		}
		values := strings.Split(line, " ")
		if len(values) != 4 {
			l.Debug("FormatError", line)
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		password, err := getMD5(values[3])
		if err != nil {
			l.Error("GetMD5", err)
		}
		users = append(users, &userdata{Surname: values[0], Name: values[1], Otch: values[2], Role: usersType, Password: password})
	}
	//

	for _, user := range users {
		// createOnlyMetauser req
		createOnlyMetauserReq, err := conf.createOnlyMetauser.CreateRequestFrom(suckhttp.POST, "", r)
		if err != nil {
			l.Error("CreateRequestFrom", err)
			return suckhttp.NewResponse(500, "Internal server error"), nil
		}
		createOnlyMetauserReq.Body = []byte(suckutils.ConcatFour("surname=", user.Surname, "&name=", user.Name))
		createOnlyMetauserReq.AddHeader(suckhttp.Content_Type, "application/x-www-form-urlencoded")
		createOnlyMetauserResp, err := conf.createOnlyMetauser.Send(createOnlyMetauserReq)
		if err != nil {
			l.Error("Send", err)
			return suckhttp.NewResponse(500, "Internal server error"), nil
		}
		if i, t := createOnlyMetauserResp.GetStatus(); i/100 != 2 {
			l.Error("Resp from createOnlyMeta", errors.New(t))
			return suckhttp.NewResponse(500, "Internal server error"), nil
		}

		if metaid := createOnlyMetauserResp.GetBody(); len(metaid) != 0 {
			user.MetaId = string(metaid)
		} else {
			l.Error("Resp from createOnlyMeta", errors.New("empty body"))
			return suckhttp.NewResponse(500, "Internal server error"), nil
		}
		//

		if usersType == 1 {
			if len(group) == 0 {
				return suckhttp.NewResponse(400, "Bad request"), nil
			}
			user.Group = group

			// createFolderWithMetauser req
			createFolderWithMetaReq, err := conf.createOnlyMetauser.CreateRequestFrom(suckhttp.PUT, "", r)
			if err != nil {
				l.Error("CreateRequestFrom", err)
				return suckhttp.NewResponse(500, "Internal server error"), nil
			}
			createFolderWithMetaReq.Body = []byte(user.MetaId)
			createFolderWithMetaReq.AddHeader(suckhttp.Content_Type, "text/plain")
			createFolderWithMetaReq.AddHeader(suckhttp.Accept, "text/plain")
			createFolderWithMetaResp, err := conf.createFolderWithMetauser.Send(createFolderWithMetaReq)
			if err != nil {
				l.Error("Send", err)
				return suckhttp.NewResponse(500, "Internal server error"), nil
			}
			if i, t := createFolderWithMetaResp.GetStatus(); i/100 != 2 {
				l.Error("Resp from createFolderWithMeta", errors.New(t))
				return suckhttp.NewResponse(500, "Internal server error"), nil
			}

			if folderid := createFolderWithMetaResp.GetBody(); len(folderid) != 0 {
				user.FolderId = string(folderid)
			} else {
				l.Error("Resp from createFolderWithMeta", errors.New("empty body"))
				return suckhttp.NewResponse(500, "Internal server error"), nil
			}
			//
		}

		// userRegistration req
		userLogin, err := getMD5(suckutils.ConcatTwo(user.Surname, user.Name))
		if err != nil {
			l.Error("GetMD5", err)
		}
		userRegistrationReq, err := conf.userRegistration.CreateRequestFrom(suckhttp.PUT, suckutils.ConcatTwo("/", userLogin), r)
		if err != nil {
			l.Error("CreateRequestFrom", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		userRegistrationReq.Body = []byte(user.Password)
		userRegistrationReq.AddHeader(suckhttp.Content_Type, "text/plain")

		userRegistrationResp, err := conf.userRegistration.Send(userRegistrationReq)
		if err != nil {
			l.Error("Send req to userregistration", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		if i, t := userRegistrationResp.GetStatus(); i/100 != 2 {
			l.Error("Resp from userregistration", errors.New(suckutils.ConcatTwo("statuscode is ", t)))
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		l.Debug(user.Surname, "Done!")
		//

		// setUserData req
		setUserDataReq, err := conf.setUserData.CreateRequestFrom(suckhttp.PUT, suckutils.ConcatTwo("/", userLogin), r)
		if err != nil {
			l.Error("CreateRequestFrom", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		setUserDataReq.AddHeader(suckhttp.Content_Type, "application/json")

		if setUserDataReq.Body, err = json.Marshal(user); err != nil {
			l.Error("Marshalling userData", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}

		setUserDataResp, err := conf.setUserData.Send(setUserDataReq)
		if err != nil {
			l.Error("Send req to setuserdata", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		if i, t := setUserDataResp.GetStatus(); i/100 != 2 {
			l.Error("Resp from setuserdata", errors.New(suckutils.ConcatTwo("statuscode is ", t)))
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		//
	}

	return suckhttp.NewResponse(200, "OK"), nil
}

func readFromFile(req *suckhttp.Request, name string) ([]byte, error) {
	_, params, err := mime.ParseMediaType(req.GetHeader("content-type"))
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(req.Body)
	mr := multipart.NewReader(reader, params["boundary"])
	var filedata []byte
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if p.FormName() != name {
			continue
		}
		filedata, err = io.ReadAll(p)
		if err != nil {
			return nil, err
		}
	}
	if filedata == nil {
		return nil, errors.New(suckutils.ConcatThree("Field ", name, " not found"))
	}
	return filedata, nil
}

func getMD5(str string) (string, error) {
	hash := md5.New()
	_, err := hash.Write([]byte(str))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
