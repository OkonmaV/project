package repoMongo

import "github.com/big-larry/mgo"

func ConnectToMongo(addr, db, col string) (*mgo.Session, *mgo.Collection, error) {
	mgoSession, err := mgo.Dial(addr)
	if err != nil {
		return nil, nil, err
	}
	return mgoSession, mgoSession.DB(db).C(col), nil
}
