package repo

import (
	"io/ioutil"
	"text/template"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
)

type User struct {
	Id     bson.ObjectId `bson:"_id,omitempty" json:"_id,omitempty"`
	Logins []string      `bson:"logins,omitempty" json:"logins,omitempty"`
}

type Data struct {
	Mail     string `bson:"mail,omitempty" json:"mail,omitempty"`
	Name     string `bson:"name,omitempty" json:"name,omitempty"`
	Surname  string `bson:"surname,omitempty" json:"surname,omitempty"`
	Otch     string `bson:"otch,omitempty" json:"otch,omitempty"`
	GroupId  string `bson:"groupid,omitempty" json:"groupid,omitempty"`
	MetaId   string `bson:"metaid,omitempty" json:"metaid,omitempty"`
	FolderId string `bson:"folderid,omitempty" json:"folderid,omitempty"`
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
