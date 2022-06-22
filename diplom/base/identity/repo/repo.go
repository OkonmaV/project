package repo

import (
	"io/ioutil"
	"text/template"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/dgrijalva/jwt-go"
	"github.com/tarantool/go-tarantool"
)

type User struct {
	Id     bson.ObjectId `bson:"_id,omitempty" json:"_id,omitempty"`
	Data   Data          `bson:"data" json:"data"`
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

type CookieClaims struct {
	Login   string `json:"login"`
	Role    int    `json:"role"`
	Surname string `json:"surname"`
	Name    string `json:"name"`
	jwt.StandardClaims
}

type TarantoolAuthTuple struct {
	Login    string
	Password string
	UserId   string
}

type TarantoolVerifyTuple struct {
	Hash string
	Uuid string
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

func ConnectToMongo(addr, db, col string) (*mgo.Session, *mgo.Collection, error) {
	mgoSession, err := mgo.Dial(addr)
	if err != nil {
		return nil, nil, err
	}
	return mgoSession, mgoSession.DB(db).C(col), nil
}

func ConnectToTarantool(trntlAddr string) (*tarantool.Connection, error) {
	return tarantool.Connect(trntlAddr, tarantool.Opts{
		// User: ,
		// Pass: ,
		Timeout:       500 * time.Millisecond,
		Reconnect:     1 * time.Second,
		MaxReconnects: 4,
	})
}
