package repo

import (
	"io/ioutil"
	"text/template"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
)

type Chat struct {
	Id    bson.ObjectId `bson:"_id"`
	Type  int           `bson:"type"`
	Users []User        `bson:"users"`
	Name  string        `bson:"name,omitempty"`
}
type User struct {
	UserId        string    `bson:"userid"`
	Type          int       `bson:"type"`
	ChatName      string    `bson:"chatname,omitempty"`
	StartDateTime time.Time `bson:"startdatetime,omitempty"`
	EndDateTime   time.Time `bson:"enddatetime,omitempty"`
}

func GetTemplate(filename string) (*template.Template, error) {
	templData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	templ, err := template.New("index").Parse(string(templData))
	if err != nil {
		return nil, err
	}
	return templ, nil

}

func ConnectToMongo(mgoAddr, dbname string) (*mgo.Session, error) {
	mgoSession, err := mgo.Dial(mgoAddr)
	if err != nil {
		return nil, err
	}
	return mgoSession, nil
}
