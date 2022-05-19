package main

import (
	"context"
	"encoding/json"
	"errors"

	"project/httpservice"
	repoMongo "project/repo/mongo"
	repoChats "project/services/chats/repo"
	"project/test/types"
	"strconv"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
)

//read this from configfile
type config struct {
	MongoAddr     string
	MongoDBname   string
	MongoCollname string
}

//your shit here
type service struct {
	mongoColl *mgo.Collection
}

// cookie
type userData struct {
	Id      string `json:"id" bson:"userid"`
	Surname string `json:"surname" `
	Name    string `json:"name" `
}

const thisServiceName httpservice.ServiceName = "chats.get"

// TODO: add query escaping
func (c *config) CreateHandler(ctx context.Context, pubs_getter httpservice.Publishers_getter) (httpservice.HTTPService, error) {
	session, col, err := repoMongo.ConnectToMongo(c.MongoAddr, c.MongoDBname, c.MongoCollname)
	if err != nil {
		panic("connect to mongo: " + err.Error())
	}
	if err := session.Ping(); err != nil {
		panic("mongo ping: " + err.Error())
	}

	return &service{mongoColl: col}, nil
}

func (s *service) Handle(r *suckhttp.Request, l types.Logger) (*suckhttp.Response, error) {
	if r.GetMethod() != suckhttp.GET { //  КАКОЙ МЕТОД?
		l.Debug("Request", "not POST")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// SKIP AUTH
	userdata := &userData{Id: "user1"}
	//

	query := bson.M{"users.id:": userdata.Id}

	q := r.Uri.Query()
	dm := q.Get("del")
	if len(dm) != 0 {
		delmode, err := strconv.ParseUint(dm, 10, 8)
		if err != nil {
			l.Error("Parsing opcode from query", err)
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		switch byte(delmode) {
		case repoChats.NoDeleted:
			query["type"] = bson.M{"$lt": 100}
		case repoChats.WithDeleted:
			break
		case repoChats.OnlyDeleted:
			query["type"] = bson.M{"$gt": 100}
		default:
			l.Error("Request", errors.New("unknown deleted chats showing mode"))
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
	}

	if chid := q.Get("chatid"); len(chid) != 0 {
		if chatid, err := bson.NewObjectIdFromHex(chid); err != nil {
			l.Error("Request", errors.New("chatId is not objectId"))
			return suckhttp.NewResponse(400, "Bad request"), nil
		} else {
			query["_id"] = chatid
		}
	}

	chats := []repoChats.Chat{}

	if err := s.mongoColl.Find(query).All(&chats); err != nil {
		return nil, err
	}

	var contentType string
	chatsjson, err := json.Marshal(chats)
	if err != nil {
		l.Error("json.Marshal", err)
		return suckhttp.NewResponse(500, "Internal server error"), nil
	}
	contentType = "application/json"

	return suckhttp.NewResponse(200, "OK").SetBody(chatsjson).AddHeader(suckhttp.Content_Type, contentType), nil
}

func (s *service) Close() error {
	s.mongoColl.Database.Session.Close()
	return nil
}

func main() {
	httpservice.InitNewService(thisServiceName, &config{}, false, 1)
}
