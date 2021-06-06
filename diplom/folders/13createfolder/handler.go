package main

import (
	"errors"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/rs/xid"
)

type CreateFolder struct {
	mgoSession *mgo.Session
	mgoColl    *mgo.Collection
}
type folder struct {
	Id      string   `bson:"_id"`
	RootsId []string `bson:"rootsid"`
	Name    string   `bson:"name"`
	Metas   []meta   `bson:"metas"`
}

type meta struct {
	Type int    `bson:"metatype"`
	Id   string `bson:"metaid"`
}

func NewCreateFolder(mgodb string, mgoAddr string, mgoColl string) (*CreateFolder, error) {

	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		logger.Error("Mongo conn", err)
		return nil, err
	}
	logger.Info("Mongo", "Connected!")
	mgoCollection := mgoSession.DB(mgodb).C(mgoColl)

	return &CreateFolder{mgoSession: mgoSession, mgoColl: mgoCollection}, nil

}

func (conf *CreateFolder) Close() error {
	conf.mgoSession.Close()
	return nil
}

func (conf *CreateFolder) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	// Мои комменты не удаляй!

	// POST без тела это нормально, вот раздув: https://stackoverflow.com/questions/4191593/is-it-considered-bad-practice-to-perform-http-post-without-entity-body

	// Здесь нам нужен PUT с URI вида "/root_folder_id?name=newfolder_name"
	// Работает как touch в linux
	// Возвращаем 201 Created с newfolder_id в теле, если папка с таким именем есть, то возвращаем 409 Conflict
	// Логика такая:
	// 1. Проверяем разрешено ли чуваку создавать папки в root_id через auth-сервис, таким образом сразу проверяется существование папки с root_id, но я бы сделал еще проверку, на случай потери целостности данных.
	// 2. Проверяем есть ли root_id в базе (запрос One с минимальным количеством полей (_id) в монгу)
	// 3. Делаем Upsert с запросом по имени папки, чтобы не клонровать одинаковые папки. Insert делается там, где нет иникального имени или типо того. Можно сделать уникалиный индекс и потом делать инсерт, но тогда мы часть логики возлагаем на БД и можем забыть создать индекс или не сделать его уникальным. Я предпочитаю сам алгоритм на разносить на разные сервисы...

	//TODO: Захуячить в mgo функцию Exists(query)

	if r.GetMethod() != suckhttp.PUT {
		return suckhttp.NewResponse(405, "Method not allowed"), nil
	}

	folderRootId := r.Uri.Path
	folderRootId = strings.Trim(folderRootId, "/")
	folderName := strings.TrimSpace(r.Uri.Query().Get("name")) // Trim всегда делай в таких моментах
	if folderName == "" || folderRootId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// TODO: AUTH

	// TODO: get metauser
	metaid := "randmetaid"
	//

	// check root
	query := &bson.M{"_id": folderRootId, "deleted": bson.M{"$exists": false}}

	if err := conf.mgoColl.Find(query).Select(bson.M{"_id": 1}).One(nil); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}

	newfolderId := xid.New()
	if newfolderId.IsNil() { // Такое может быть? Надо в New посмотреть... Я пока не могу :(
		return nil, errors.New("get new rand id returned nil")
	}
	query = &bson.M{"name": folderName, "rootsid": folderRootId, "deleted": bson.M{"$exists": false}}
	change := &bson.M{"$setOnInsert": &folder{Id: newfolderId.String(), RootsId: []string{folderRootId}, Name: folderName, Metas: []meta{{Type: 0, Id: metaid}}}}

	changeInfo, err := conf.mgoColl.Upsert(query, change)
	if err != nil {
		return nil, err
	}
	if changeInfo.Matched != 0 {
		return suckhttp.NewResponse(409, "Conflict"), nil
	}
	resp := suckhttp.NewResponse(201, "Created")
	if strings.Contains(r.GetHeader(suckhttp.Accept), "text/plain") {
		resp.SetBody(newfolderId.Bytes())
	}
	return resp, nil
}
