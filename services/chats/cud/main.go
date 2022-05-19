package main

import (
	"context"
	"errors"
	"project/httpservice"
	repoMongo "project/repo/mongo"
	repoChats "project/services/chats/repo"
	"project/test/types"
	"strconv"
	"strings"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
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
	Id      string `json:"id"`
	Surname string `json:"surname" `
	Name    string `json:"name" `
}

const thisServiceName httpservice.ServiceName = "chats.cud"

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
	if r.GetMethod() != suckhttp.POST { //  КАКОЙ МЕТОД?
		l.Debug("Request", "not POST")
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	// SKIP AUTH
	userdata := &userData{Id: "user1", Surname: "surname1", Name: "name1"}
	//
	uriquery := r.Uri.Query()
	opcode, err := strconv.ParseUint(uriquery.Get("op"), 10, 8)
	if err != nil {
		l.Error("Parsing opcode from query", err)
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var query, update bson.M
	switch byte(opcode) {

	case repoChats.OpCreate:
		ct, err := strconv.ParseUint(uriquery.Get("newchattype"), 10, 8)
		if err != nil {
			l.Error("Parsing new chat type", err)
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		newchattype := repoChats.ChatType(ct)

		switch newchattype {
		case repoChats.ChatTypeSingle:
			chatName := r.Uri.Query().Get("chatname")
			if len(chatName) == 0 {
				chatName = suckutils.ConcatThree(userdata.Surname, " ", userdata.Name)
			}
			query = bson.M{"type": newchattype, "users.0.userid": "userId", "users.0.type": repoChats.AdminRights, "name": chatName}
			update = bson.M{"$setOnInsert": &repoChats.Chat{Id: bson.NewObjectId(), Type: newchattype, Users: []repoChats.ChatUser{{Id: userdata.Id, Type: repoChats.AdminRights, StartDateTime: time.Now()}}}}

		case repoChats.ChatTypeTwo:
			secondUserId := r.Uri.Query().Get("seconduser")
			if len(secondUserId) == 0 {
				l.Debug("Request", "query param \"seconduser\" is empty")
				return suckhttp.NewResponse(400, "Bad request"), nil
			}
			// SKIP GETTING SECOND USER's DATA
			seconduserdata := &userData{Id: "user2", Surname: "surname2", Name: "name2"}
			//

			users := []repoChats.ChatUser{{Id: userdata.Id, ChatName: suckutils.ConcatThree(seconduserdata.Surname, " ", seconduserdata.Name), Type: repoChats.AdminRights, StartDateTime: time.Now()}, {Id: seconduserdata.Id, ChatName: suckutils.ConcatThree(userdata.Surname, " ", userdata.Name), Type: repoChats.AdminRights, StartDateTime: time.Now()}}
			query = bson.M{"type": newchattype, "$or": []bson.M{{"users.0.userid": userdata.Id, "users.1.userid": seconduserdata.Id}, {"users.0.userid": seconduserdata.Id, "users.1.userid": userdata.Id}}}
			update = bson.M{"$setOnInsert": &repoChats.Chat{Id: bson.NewObjectId(), Type: newchattype, Users: users}}

		case repoChats.ChatTypeGroup:
			chatName := r.Uri.Query().Get("chatname")
			if len(chatName) == 0 {
				chatName = "Group chat"
			}
			//alternative: query = bson.M{"type": chatType, "users": bson.M{"$elemMatch": bson.M{"userid": userId, "type": 0}}, "name": chatName}
			query = bson.M{"type": newchattype, "users.0.userid": "userId", "users.0.type": repoChats.AdminRights, "name": chatName}
			update = bson.M{"$setOnInsert": &repoChats.Chat{Id: bson.NewObjectId(), Type: newchattype, Users: []repoChats.ChatUser{{Id: userdata.Id, Type: repoChats.AdminRights, StartDateTime: time.Now()}}}}

		default:
			l.Error("Request", errors.New("unknown new chat type"))
			return suckhttp.NewResponse(400, "Bad request"), nil
		}

		change := mgo.Change{
			Update:    update,
			Upsert:    true,
			ReturnNew: true,
			Remove:    false,
		}
		var mgoRes map[string]string
		changeInfo, err := s.mongoColl.Find(query).Select(bson.M{"_id": 1}).Apply(change, &mgoRes)
		if err != nil {
			return suckhttp.NewResponse(500, "Internal Server Error "), nil
		}
		if changeInfo.Matched == 0 {
			return suckhttp.NewResponse(201, "Created").SetBody([]byte(mgoRes["_id"])), nil
		} else {
			return suckhttp.NewResponse(200, "OK").SetBody([]byte(mgoRes["_id"])), nil
		}

	case repoChats.OpRename, repoChats.OpDelete:
		chatId, err := bson.NewObjectIdFromHex(strings.Trim(r.Uri.Path, "/"))
		if err != nil {
			l.Debug("Request", "chatId (path) is nil or not objectId")
			return suckhttp.NewResponse(400, "Bad request"), nil
		}
		query = bson.M{"_id": chatId, "users": bson.M{"$elemMatch": bson.M{"userid": userdata.Id, "type": repoChats.AdminRights}}}
		change := mgo.Change{
			Upsert:    false,
			ReturnNew: false,
			Remove:    false,
		}

		if byte(opcode) == repoChats.OpRename {
			chatName := r.Uri.Query().Get("chatname")
			if len(chatName) == 0 {
				l.Debug("Request", "no specified chatname to rename to")
				return suckhttp.NewResponse(400, "Bad request"), nil
			}
			change.Update = bson.M{"$set": bson.M{"name": chatName}}
		} else {
			query["type"] = bson.M{"$lt": 100}
			change.Update = bson.M{"$inc": bson.M{"type": 100}}
		}

		if _, err := s.mongoColl.Find(query).Apply(change, nil); err != nil {
			if err == mgo.ErrNotFound {
				l.Debug("FindAndModify", "not found (no chat with this id or dont have permissions)")
				return suckhttp.NewResponse(403, "Forbidden"), nil
			}
			return nil, err
		}

	default:
		l.Error("Request", errors.New(suckutils.ConcatTwo("unknown operation code: ", strconv.FormatUint(opcode, 10))))
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	return suckhttp.NewResponse(200, "OK"), nil
}

func (s *service) Close() error {
	s.mongoColl.Database.Session.Close()
	return nil
}

func main() {
	httpservice.InitNewService(thisServiceName, &config{}, false, 1)
}
