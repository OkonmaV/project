package main

import (
	"errors"
	"net/url"
	"project/diplom/folders/repo"
	"strconv"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type Handler struct {
	mgoColl *mgo.Collection
	auth    *httpservice.Authorizer
	authSet *httpservice.InnerService
}

type authreqdata struct {
	Login  string `json:"login"`
	Metaid string `json:"metaid"`
	UserId bson.ObjectId
}

func NewHandler(col *mgo.Collection, auth *httpservice.InnerService, authSet *httpservice.InnerService, tokendecoder *httpservice.InnerService) (*Handler, error) {
	authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{mgoColl: col, auth: authorizer, authSet: authSet}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	// Мои комменты не удаляй!

	// POST без тела это нормально, вот раздув: https://stackoverflow.com/questions/4191593/is-it-considered-bad-practice-to-perform-http-post-without-entity-body

	// Здесь нам нужен PUT с URI вида "/root_folder_id?name=newfolder_name"
	// Работает как touch в linux
	// Возвращаем 201 Created с newfolder_id в теле, если папка с таким именем есть, то возвращаем 409 Conflict
	// Логика такая:
	// 1. Проверяем разрешено ли чуваку создавать папки в root_id через auth-сервис, таким образом сразу проверяется существование папки с root_id, но я бы сделал еще проверку, на случай потери целостности данных.
	// 2. Проверяем есть ли root_id в базе (запрос One с минимальным количеством полей (_id) в монгу)
	// 3. Делаем Upsert с запросом по имени папки, чтобы не клонровать одинаковые папки. Insert делается там, где нет иникального имени или типо того. Можно сделать уникалиный индекс и потом делать инсерт, но тогда мы часть логики возлагаем на БД и можем забыть создать индекс или не сделать его уникальным. Я предпочитаю сам алгоритм на разносить на разные сервисы...

	//TODO: Сделать в mgo функцию Exists(query)

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || r.GetMethod() != suckhttp.PUT {
		l.Debug("Request", "method or content-type not allowed")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	folderRootId, err := bson.NewObjectIdFromHex(strings.Trim(r.Uri.Path, "/"))
	if err != nil {
		l.Debug("Request", "rootid not specified in path")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		l.Debug("Parsing body", err.Error())
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	folderName := formValues.Get("name")
	folderType, err := strconv.Atoi(formValues.Get("type"))
	if folderName == "" || err != nil {
		l.Debug("Request", "name or type not specified correctly in body")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// metadata
	folderInfo := formValues.Get("info")
	//

	//AUTH
	userData := &authreqdata{}
	k, err := conf.auth.GetAccessWithData(r, l, "folders", 1, userData)
	if err != nil {
		return nil, err
	}
	if !k {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	if userData.Metaid == "" {
		l.Error("GetAccessWithData", errors.New("no metaid in resp"))
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	//

	// checking root
	query := bson.M{"_id": folderRootId, "deleted": bson.M{"$exists": false}}

	if err := conf.mgoColl.Find(query).Select(bson.M{"_id": 1}).One(nil); err != nil {
		if err == mgo.ErrNotFound {
			l.Debug("Checking root", "root not found")
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	//TODO: get userid (now its userData.UserId)
	query = bson.M{"name": folderName, "rootsid": folderRootId, "deleted": bson.M{"$exists": false}}
	change := &bson.M{"$setOnInsert": &repo.Folder{RootsId: []bson.ObjectId{folderRootId}, Name: folderName, Type: folderType, Users: []repo.User{{Type: 1, Id: userData.UserId}}, Metadata: repo.Metadata{Info: folderInfo}}}

	changeInfo, err := conf.mgoColl.Upsert(query, change)
	if err != nil {
		return nil, err
	}
	if changeInfo.Matched != 0 {
		return suckhttp.NewResponse(409, "Conflict"), nil
	}

	resp := suckhttp.NewResponse(201, "Created")
	if strings.Contains(r.GetHeader(suckhttp.Accept), "text/plain") {
		resp.SetBody([]byte(changeInfo.UpsertedId.(bson.ObjectId).Hex())).AddHeader(suckhttp.Content_Type, "text/plain")
	}
	return resp, nil
}
