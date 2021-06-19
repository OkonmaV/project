package main

import (
	"errors"
	"net/url"
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
type folder struct {
	Id         string   `bson:"_id"`
	RootsId    []string `bson:"rootsid"`
	Name       string   `bson:"name"`
	Type       int      `bson:"type"`
	Metas      []meta   `bson:"metas"`
	Speciality string   `bson:"speciality"`
}

type meta struct {
	Type int    `bson:"metatype"`
	Id   string `bson:"metaid"`
}

type authreqdata struct {
	Login  string `json:"Login"`
	Metaid string `json:"metaid"`
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
		return suckhttp.NewResponse(405, "Method not allowed"), nil
	}

	folderRootId := strings.Trim(r.Uri.Path, "/")

	formValues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		l.Error("Parsing r.Body", err)
		return suckhttp.NewResponse(401, "Bad Request"), nil
	}
	folderName := formValues.Get("name")
	folderType, err := strconv.Atoi(formValues.Get("type")) // 5 - вкр, 4 - группа
	if folderName == "" || folderRootId == "" || err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	var folderSpeciality string
	if folderType == 4 {
		folderSpeciality = formValues.Get("speciality")
		if folderSpeciality == "" {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
	}
	//check auth
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

	// checking root
	query := &bson.M{"_id": folderRootId, "deleted": bson.M{"$exists": false}}

	if err := conf.mgoColl.Find(query).Select(bson.M{"_id": 1}).One(nil); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	newfolderId := bson.NewObjectId().Hex()

	// //set auth for new folder
	// authSetReq, err := conf.authSet.CreateRequestFrom(suckhttp.POST, suckutils.Concat("/", userData.Login, ".", newfolderId, "?perm=1"), r)
	// if err != nil {
	// 	l.Error("CreateRequestFrom", err)
	// 	return suckhttp.NewResponse(500, "Internal Server Error"), nil
	// }
	// authSetResp, err := conf.authSet.Send(authSetReq)
	// if err != nil {
	// 	l.Error("Send", err)
	// 	return suckhttp.NewResponse(500, "Internal Server Error"), nil
	// }
	// if i, t := authSetResp.GetStatus(); i/100 != 2 {
	// 	l.Debug("Resp from auth", t)
	// 	return suckhttp.NewResponse(500, "Internal Server Error"), nil
	// }

	query = &bson.M{"name": folderName, "rootsid": folderRootId, "deleted": bson.M{"$exists": false}}
	change := &bson.M{"$setOnInsert": &folder{Id: newfolderId, Speciality: folderSpeciality, RootsId: []string{folderRootId}, Name: folderName, Type: folderType, Metas: []meta{{Type: 0, Id: userData.Metaid}}}}

	changeInfo, err := conf.mgoColl.Upsert(query, change)
	if err != nil {
		return nil, err
	}
	if changeInfo.Matched != 0 {
		return suckhttp.NewResponse(409, "Conflict"), nil
	}
	resp := suckhttp.NewResponse(201, "Created")
	if strings.Contains(r.GetHeader(suckhttp.Accept), "text/plain") {
		resp.SetBody([]byte(newfolderId))
	}
	return resp, nil
}
