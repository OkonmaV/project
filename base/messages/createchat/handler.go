package main

import (
	"encoding/json"
	"errors"
	"strconv"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type chat struct {
	Id    string `bson:"_id"`
	Type  int    `bson:"type"`
	Users []user `bson:"users"`
	Name  string `bson:"name,omitempty"`
}
type user struct {
	UserId        string    `bson:"userid"`
	Type          int       `bson:"type"`
	ChatName      string    `bson:"chatname,omitempty"`
	StartDateTime time.Time `bson:"startdatetime"`
	//EndDateTime   time.Time `bson:"enddatetime"`
}

type cookieData struct {
	UserId  string `json:"Login"`
	Surname string `json:"surname"`
	Name    string `json:"name"`
}

type Handler struct {
	mgoColl      *mgo.Collection
	getUserData  *httpservice.InnerService
	tokenDecoder *httpservice.InnerService
}

func NewHandler(col *mgo.Collection, getUserData *httpservice.InnerService, tokendecoder *httpservice.InnerService) (*Handler, error) {

	return &Handler{mgoColl: col, tokenDecoder: tokendecoder, getUserData: getUserData}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST { //  КАКОЙ МЕТОД?
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	//AUTH
	koki, ok := r.GetCookie("koki")
	if !ok || len(koki) < 5 {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	tokenDecoderReq, err := conf.tokenDecoder.CreateRequestFrom(suckhttp.GET, suckutils.Concat("/", koki), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	tokenDecoderReq.AddHeader(suckhttp.Accept, "application/json")
	tokenDecoderResp, err := conf.tokenDecoder.Send(tokenDecoderReq)
	if err != nil {
		l.Error("Send", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if i, t := tokenDecoderResp.GetStatus(); i/100 != 2 {
		l.Debug("Resp from tokendecoder", t)
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	if len(tokenDecoderResp.GetBody()) == 0 {
		l.Debug("Resp from tokendecoder", "empty body")
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	userData := &cookieData{}

	if err = json.Unmarshal(tokenDecoderResp.GetBody(), userData); err != nil {
		l.Error("Unmarshal", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if userData.Name == "" || userData.UserId == "" || userData.Surname == "" {
		// TODO: delete cookie here
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	//

	chatType, err := strconv.Atoi(r.Uri.Query().Get("chattype"))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	var query bson.M
	var update bson.M
	switch chatType {
	case 1: //tet-a-tet

		withUserId := r.Uri.Query().Get("withid")
		if withUserId == "" {
			return suckhttp.NewResponse(400, "Bad request"), nil
		}

		getUserDataReq, err := conf.getUserData.CreateRequestFrom(suckhttp.GET, suckutils.ConcatThree("/", withUserId, "?fields=surname&fields=name"), r)
		if err != nil {
			l.Error("CreateRequestFrom", err)
			return suckhttp.NewResponse(500, "Internal server error"), nil
		}
		getUserDataReq.AddHeader(suckhttp.Accept, "application/json")
		getUserDataResp, err := conf.getUserData.Send(getUserDataReq)
		if err != nil {
			l.Error("Send", err)
			return suckhttp.NewResponse(500, "Internal server error"), nil
		}

		if i, t := getUserDataResp.GetStatus(); i != 200 {
			l.Debug("Responce from getuserdata", t)
			return suckhttp.NewResponse(i, t), nil
		}

		withUserData := make(map[string]string)
		if err = json.Unmarshal(getUserDataResp.GetBody(), &withUserData); err != nil {
			l.Error("Unmarshalling getuserdata resp", err)
			return suckhttp.NewResponse(500, "Internal server error"), nil
		}
		if withUserData["surname"] == "" || withUserData["name"] == "" {
			l.Error("Getuserdata resp", errors.New("empty requested data"))
			return suckhttp.NewResponse(500, "Internal server error"), nil
		}
		users := []user{{UserId: userData.UserId, ChatName: suckutils.ConcatThree(withUserData["surname"], " ", withUserData["name"]), Type: 1, StartDateTime: time.Now()}, {UserId: withUserId, ChatName: suckutils.ConcatThree(userData.Surname, " ", userData.Name), Type: 1, StartDateTime: time.Now()}}
		//alternative: query = bson.M{"type": chatType, "users": bson.M{"$all": []bson.M{{"$elemMatch": bson.M{"userid": userId}}, {"$elemMatch": bson.M{"userid": withUserId}}}}}
		query = bson.M{"type": chatType, "$or": []bson.M{{"users.0.userid": userData.UserId, "users.1.userid": withUserId}, {"users.0.userid": withUserId, "users.1.userid": userData.UserId}}}
		update = bson.M{"$setOnInsert": &chat{Id: bson.NewObjectId().Hex(), Type: chatType, Users: users}}

	case 2: //group

		chatName := r.Uri.Query().Get("chatname")
		if chatName == "" {
			chatName = "Group chat"
		}

		//alternative: query = bson.M{"type": chatType, "users": bson.M{"$elemMatch": bson.M{"userid": userId, "type": 0}}, "name": chatName}
		query = bson.M{"type": chatType, "users.0.userid": "userId", "users.0.type": 0, "name": chatName}
		update = bson.M{"$setOnInsert": &chat{Id: bson.NewObjectId().Hex(), Type: chatType, Users: []user{{UserId: userData.UserId, Type: 1, StartDateTime: time.Now()}}}}

	default:
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	change := mgo.Change{
		Update:    update,
		Upsert:    true,
		ReturnNew: true,
		Remove:    false,
	}

	var mgoRes map[string]string
	changeInfo, err := conf.mgoColl.Find(query).Select(bson.M{"_id": 1}).Apply(change, &mgoRes)
	if err != nil {
		return nil, err
	}
	if changeInfo.Matched == 0 {
		return suckhttp.NewResponse(201, "Created").SetBody([]byte(mgoRes["_id"])), nil
	} else {
		return suckhttp.NewResponse(200, "OK").SetBody([]byte(mgoRes["_id"])), nil
	}
	// ??????????????? REDIRECT?????????

}
