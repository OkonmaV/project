package main

import (
	"encoding/json"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type Handler struct {
	mgoColl *mgo.Collection
}

// Варнинг: через потенциальный сервис изменения пользовательских данных (при подмене куки и хреновой авторизации) есть вероятность создать левый акк в обход регистрации (без записи в тарантуле для аутентификации, но все же)

func NewHandler(mgocol *mgo.Collection) (*Handler, error) {

	return &Handler{mgoColl: mgocol}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/json") || r.GetMethod() != suckhttp.PUT || len(r.Body) == 0 {
		l.Debug("Request", "not PUT or content-type not application/json or body is empty")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	query := bson.M{"deleted": bson.M{"$exists": false}}
	update := make(bson.M)

	userLogin := strings.TrimSpace(r.Uri.Query().Get("login"))
	if len(userLogin) != 32 && len(userLogin) > 0 {
		l.Debug("Request", "userLogin (query param \"login\") was not specified correctly")
		return suckhttp.NewResponse(400, "Bad request"), nil
	} else {
		update["$setOnInsert"] = bson.M{"logins": userLogin}
	}

	if userId, err := bson.NewObjectIdFromHex(strings.Trim(r.Uri.Path, "/")); err == nil {
		query["_id"] = userId
	} else if len(userLogin) == 32 {
		query["logins"] = userLogin
	} else {
		l.Debug("Request", "non of userLogin (query param \"login\") or userId (path) were correctly specified")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var upsertData map[string]interface{}
	if err := json.Unmarshal(r.Body, &upsertData); err != nil {
		l.Error("Unmarshalling r.Body", err)
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	update["$set"] = bson.M{"data": &upsertData}

	changeInfo, err := conf.mgoColl.Upsert(query, update)
	if err != nil {
		return nil, err
	}
	if changeInfo.Matched == 0 {
		if strings.Contains(r.GetHeader(suckhttp.Accept), "text/plain") {
			body := []byte(changeInfo.UpsertedId.(bson.ObjectId).Hex())
			return suckhttp.NewResponse(201, "Created").AddHeader(suckhttp.Content_Type, "text/plain").SetBody(body), nil
		}
		return suckhttp.NewResponse(201, "Created"), nil
	}

	return suckhttp.NewResponse(200, "OK"), nil

}
