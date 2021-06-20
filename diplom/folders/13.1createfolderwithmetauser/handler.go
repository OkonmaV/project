package main

import (
	"net/url"
	"strconv"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type Handler struct {
	mgoColl *mgo.Collection
	//auth    *httpservice.Authorizer
	//authSet *httpservice.InnerService
}
type folder struct {
	Id      string   `bson:"_id"`
	RootsId []string `bson:"rootsid"`
	Name    string   `bson:"name"`
	Type    int      `bson:"type"`
	Metas   []meta   `bson:"metas"`
}

type meta struct {
	Type int    `bson:"metatype"`
	Id   string `bson:"metaid"`
}

type authreqdata struct {
	Login  string `json:"Login"`
	Metaid string `json:"metaid"`
}

func NewHandler(col *mgo.Collection /*, auth *httpservice.InnerService, authSet *httpservice.InnerService, tokendecoder *httpservice.InnerService*/) (*Handler, error) {
	// authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	// if err != nil {
	// 	return nil, err
	// }
	return &Handler{mgoColl: col /*, auth: authorizer, authSet: authSet*/}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.PUT {
		return suckhttp.NewResponse(405, "Method not allowed"), nil
	}

	folderRootId := strings.Trim(r.Uri.Path, "/")
	if folderRootId == "" {
		folderRootId = "root"
	}
	folderName := strings.TrimSpace(r.Uri.Query().Get("name"))
	if folderName == "" {
		folderName = "default"
	}
	var metaId string
	var metatype int
	var foldertype int
	if strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") {
		formValues, err := url.ParseQuery(string(r.Body))
		if err != nil {
			l.Debug("ParseQuery in body", err.Error())
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		metaId = formValues.Get("metaid")
		metatype, err = strconv.Atoi(formValues.Get("metatype"))
		if err != nil {
			l.Debug("Atoi", err.Error())
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		foldertype, err = strconv.Atoi(formValues.Get("foldertype"))
		if err != nil {
			l.Debug("Atoi", err.Error())
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
	} else {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	query := &bson.M{"_id": folderRootId, "deleted": bson.M{"$exists": false}}

	if err := conf.mgoColl.Find(query).Select(bson.M{"_id": 1}).One(nil); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	newfolderId := bson.NewObjectId().Hex()

	//query = &bson.M{"rootsid": folderRootId, "deleted": bson.M{"$exists": false}}

	err := conf.mgoColl.Insert(&folder{Id: newfolderId, Type: foldertype, RootsId: []string{folderRootId}, Name: folderName, Metas: []meta{{Type: metatype, Id: metaId}}})
	if err != nil {
		return nil, err
	}
	resp := suckhttp.NewResponse(201, "Created")
	if strings.Contains(r.GetHeader(suckhttp.Accept), "text/plain") {
		resp.SetBody([]byte(newfolderId))
	}
	return resp, nil
}
