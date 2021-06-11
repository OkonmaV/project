package main

import (
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

type Handler struct {
	auth    *httpservice.Authorizer
	mgoColl *mgo.Collection
}

func NewHandler(col *mgo.Collection, auth *httpservice.InnerService, tokendecoder *httpservice.InnerService) (*Handler, error) {
	authorizer, err := httpservice.NewAuthorizer(thisServiceName, auth, tokendecoder)
	if err != nil {
		return nil, err
	}
	return &Handler{mgoColl: col, auth: authorizer}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.DELETE {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	fid := strings.Trim(r.Uri.Path, "/")
	deletionMetaId := r.Uri.Query().Get("fdeletemetaid")
	if fid == "" || deletionMetaId == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	k, _, err := conf.auth.GetAccess(r, l, "folders", 1)
	if err != nil {
		return nil, err
	}
	if !k {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	query := &bson.M{"_id": fid, "deleted": bson.M{"$exists": false}, "metas": bson.M{"$not": bson.M{"$eq": bson.M{"metaid": deletionMetaId, "metatype": 0}}}}

	change := mgo.Change{
		Update:    bson.M{"$pull": bson.M{"metas": bson.M{"metaid": deletionMetaId}}},
		Upsert:    false,
		ReturnNew: false,
		Remove:    false,
	}

	if _, err := conf.mgoColl.Find(query).Apply(change, nil); err != nil {
		if err == mgo.ErrNotFound {
			return suckhttp.NewResponse(403, "Forbidden"), nil
		}
		return nil, err
	}
	// если ничего не удалило, т.к. метаюзера не было в массиве, то проверки на это нет и все равно 200
	return suckhttp.NewResponse(200, "OK"), nil
}
