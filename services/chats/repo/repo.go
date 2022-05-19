package repoChats

import (
	"errors"
	"time"

	"github.com/big-larry/mgo/bson"
)

type ChatsList struct {
	Chats []Chat `json:"list"`
}

type Chat struct {
	Id    bson.ObjectId `bson:"_id" json:"id"`
	Type  ChatType      `bson:"type" json:"type"`
	Users []ChatUser    `bson:"users" json:"users"`
	Name  string        `bson:"name,omitempty" json:""`
}
type ChatUser struct {
	Id            string    `bson:"id" json:"id"`
	Type          byte      `bson:"type" json:"type"`
	ChatName      string    `bson:"chatname,omitempty" json:"chatname"`
	StartDateTime time.Time `bson:"startdatetime,omitempty" json:"startdatetime,omitempty"`
	EndDateTime   time.Time `bson:"enddatetime,omitempty" json:"enddatetime,omitempty"`
}

type ChatType byte

const (
	ChatTypeDeletedSingle ChatType = 1
	ChatTypeDeletedTwo    ChatType = 2
	ChatTypeDeletedGroup  ChatType = 3
	ChatTypeSingle        ChatType = 101
	ChatTypeTwo           ChatType = 102
	ChatTypeGroup         ChatType = 103
)

func (ct ChatType) Check() error {
	switch ct {
	case ChatTypeSingle:
		return nil
	case ChatTypeTwo:
		return nil
	case ChatTypeGroup:
		return nil
	}
	return errors.New("unknown chat type")
}

func (ct ChatType) ConvertToDeleted() ChatType {
	if err := ct.Check(); err != nil {
		return 0
	}
	return ct - 100
}

// modificators for upsert
const (
	OpCreate byte = 1
	OpRename byte = 2
	OpDelete byte = 3
)

// read modificators
const (
	NoDeleted   byte = 1
	WithDeleted byte = 2
	OnlyDeleted byte = 3
)

// user rights
const (
	AdminRights byte = 1
)
