package repo

import (
	"io/ioutil"
	"text/template"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
)

type Folder struct {
	Id       bson.ObjectId   `bson:"_id"`
	RootsId  []bson.ObjectId `bson:"rootsid"`
	Name     string          `bson:"name"`
	Type     int             `bson:"type"`
	Users    []User          `bson:"metas"`
	Metadata Metadata        `bson:"metadata"`
}

type User struct {
	Type int           `bson:"type"`
	Id   bson.ObjectId `bson:"userid"`
}

type Metadata struct {
	Info string `bson:"info"`
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

// func CheckRecording(mgoColl *mgo.Collection, id bson.ObjectId) (bool, error) {
// 	if err := mgoColl.Find(bson.M{"_id": id}).Select(bson.M{"_id": 1}).One(nil); err != nil {
// 		if err == mgo.ErrNotFound {
// 			return false, nil
// 		}
// 		return false, err
// 	}
// 	return true, nil
// }
