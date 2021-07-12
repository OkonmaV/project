package repo

import (
	"errors"
	"io/ioutil"
	"text/template"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
)

// Quizes collection
type Quiz struct {
	Id        bson.ObjectId       `bson:"_id" json:"quizid"`
	Name      string              `bson:"name" json:"quizname"`
	Questions map[string]Question `bson:"questions" json:"questions"`
	CreatorId string              `bson:"creatorid" json:"creatorid"`
}

type Question struct {
	Type     int               `bson:"type" json:"type"`
	Position int               `bson:"position" json:"position"`
	Text     string            `bson:"text" json:"text"`
	Answers  map[string]string `bson:"answers" json:"answers"`
}

//

// Results collection
type Results struct {
	Id       bson.ObjectId       `bson:"_id" json:"id"`
	QuizId   string              `bson:"quizid" json:"quizid"`
	EntityId string              `bson:"entityid" json:"entityid"`
	UserId   string              `bson:"userid" json:"userid"`
	Answers  map[string][]string `bson:"answers" json:"answers"`
	Datetime time.Time           `bson:"datetime" json:"datetime"`
}

//

func GetQuiz(quizId string, mgoColl *mgo.Collection) (*Quiz, error) {

	var mgoRes Quiz
	if err := mgoColl.Find(bson.M{"_id": quizId, "deleted": bson.M{"$exists": false}}).One(&mgoRes); err != nil {
		return nil, err
	}
	return &mgoRes, nil
}

func GetQuizResults(quizId string, userId string, entityId string, mgoColl *mgo.Collection) ([]Results, error) {

	query := make(map[string]interface{})

	if quizId != "" {
		query["quizid"] = quizId
	}
	if userId != "" {
		query["userid"] = userId
	}
	if entityId != "" {
		query["entityid"] = entityId
	}

	if len(query) == 0 {
		return nil, errors.New("no params set")
	}

	var mgoRes []Results

	if err := mgoColl.Find(query).All(&mgoRes); err != nil {
		return nil, err
	}
	return mgoRes, nil
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
