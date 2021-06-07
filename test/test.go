package main

import (
	"fmt"
	"lib"
	"time"

	"github.com/big-larry/mgo"
	"github.com/big-larry/mgo/bson"
	"github.com/rs/xid"
	"github.com/tarantool/go-tarantool"
)

type chatInfo struct {
	Id    string   `bson:"_id"`
	Users []string `bson:"users"`
	Name  []string `bson:"name"`
	Type  int      `bson:"type"`
}
type Tuple struct {
	Id string
}

type flags struct {
	trntlAddr  string
	trntlTable string
}
type Testconn struct {
	conn *string
}

func changer() (r *lib.Cookie) {
	r = &lib.Cookie{}
	return r
}

type codesTuple struct {
	MetaId string `msgpack:"metaid"`
}
type folder struct {
	//Id bson.ObjectId `bson:"_id"`
	//Id    string   `bson:"_id" json:"_id,omitempty"`
	Roots []string `bson:"users,omitempty" json:"users,omitempty"`
	Name  string   `bson:"name,omitempty" json:"name,omitempty"`
	Metas []meta   `bson:"metas,omitempty" json:"metas,omitempty"`
	Time  string   `bson:"time,omitempty" json:"time,omitempty"`
}

type meta struct {
	Type []int  `json:"type"`
	Id   string `json:"id,omitempty"`
	time time.Time
}

type chat struct {
	Id    string `bson:"_id"`
	Type  int    `bson:"type"`
	Users []user `bson:"users"`
	Name  string `bson:"name,omitempty"`
}
type user struct {
	UserId        string    `bson:"userid"`
	Type          int       `bson:"type"`
	StartDateTime time.Time `bson:"startdatetime"`
	EndDateTime   time.Time `bson:"enddatetime,omitempty"`
}
type tuple struct {
	Code     int
	Hash     string
	Data     string
	MetaId   string
	Surname  string
	Name     string
	Password string
	Status   int
}

func main() {

	trntlConn, err := tarantool.Connect("127.0.0.1:3301", tarantool.Opts{
		// User: ,
		// Pass: ,
		Timeout:       500 * time.Millisecond,
		Reconnect:     1 * time.Second,
		MaxReconnects: 4,
	})
	// fmt.Println("errConn: ", err)
	// //ertrt := &tarantool.Error{Msg: suckutils.ConcatThree("Duplicate key exists in unique index 'primary' in space '", "regcodes", "'"), Code: tarantool.ErrTupleFound}

	err = trntlConn.UpsertAsync("regcodes", []interface{}{28258, "123", "asd", "asd"}, []interface{}{[]interface{}{"=", "metaid", "NEWMETAID1"}}).Err()
	fmt.Println("errUpsert:", err)

	var trntlRes tuple
	err = trntlConn.UpsertAsync("auth", []interface{}{"login", "password"}, []interface{}{[]interface{}{"=", "password", "password"}}).Err()
	//err = trntlConn.UpdateAsync("regcodes", "primary", []interface{}{28258}, []interface{}{[]interface{}{"=", "metaid", "metaid"}}).Err()
	//trntlConn.GetTyped("regcodes", "primary", []interface{}{28258}, &trntlRes)
	fmt.Println("err:", err)
	fmt.Println("resTrntl:", trntlRes)
	fmt.Println()

	//err = trntlConn.SelectTyped("regcodes", "primary", 0, 1, tarantool.IterEq, []interface{}{28258}, &trntlRes)
	// //_, err = trntlConn.Update("regcodes", "primary", []interface{}{28258}, []interface{}{[]interface{}{"=", "metaid", "h"}, []interface{}{"=", "metaname", "hh"}})

	mgoSession, err := mgo.Dial("127.0.0.1")
	if err != nil {
		return
	}
	mgoColl := mgoSession.DB("messages").C("chats")
	//ffolder := &folder{Id: "7777", Name: "NAME"}
	//ffol := &folder2{Id: &ffolder.Id, Name: &ffolder.Name, Time: &ffolder.Time}
	//err = mgoColl.Insert(ffolder)
	//fmt.Println("errinsert: ", err)

	//query2 := bson.M{"type": 1, "users": bson.M{"$all": []bson.M{{"$elemMatch": bson.M{"userid": "withUserId"}}, {"$elemMatch": bson.M{"userid": "userId"}}}}}
	//query2 := bson.M{"type": 1, "$or": []bson.M{{"users.0.userid": "withUserId", "users.1.userid": "userId"}, {"users.0.userid": "userId", "users.1.userid": "withUserId"}}}
	var upsertData map[string]interface{}
	query2 := bson.M{"_id": "c2tg1et5ddd4n3riknr0", "users.userid": "testid"} //bson.M{"$elemMatch": bson.M{"userid": "userId", "type": bson.M{"$ne": 1}}}}
	var mgores map[string]interface{}

	err = mgoColl.Find(query2).Select(bson.M{"users.$": 1}).One(&mgores)
	fmt.Println("errfind: ", err)
	fmt.Println("mgores: ", mgores)

	update := bson.M{"$set": bson.M{"data.AAAAAAAAAAAAAAAAA.FFFF": &upsertData}}
	change2 := mgo.Change{
		Update:    update, //bson.M{"$setOnInsert": &chat{Id: xid.New().String(), Type: 1, Users: []user{{UserId: "userId", Type: 0, StartDateTime: time.Now()}, {UserId: "withUserId", Type: 0, StartDateTime: time.Now()}}}},
		Upsert:    true,
		ReturnNew: true,
		Remove:    false,
	}
	var mgoRes map[string]interface{}
	_, _ = mgoColl.Find(query2).Apply(change2, &mgoRes)
	fmt.Println(upsertData == nil)

	mapa := make(map[string]interface{})
	mapa["f"] = "somestring"
	stringa := []byte(mapa["f"].(string))
	fmt.Println("byte: ", stringa)
	fmt.Println("string: ", string(stringa))
	fmt.Println("byte: ", bson.NewObjectId())
	fmt.Println("byte: ", xid.New().String())
	fmt.Println("byte: ", bson.NewObjectId().Hex())

	// err = nil
	// //bar := structs.Map(ffolder)
	// //var b
	// var inInterface map[string]interface{}
	// inrec, _ := json.Marshal(ffolder)

	// json.Unmarshal(inrec, &inInterface)

	// fmt.Println("map: ", &inInterface)
	// selector := &bson.M{"_id": "7777"} //, "metas": bson.M{"$not": bson.M{"$eq": bson.M{"id": "metaidd", "type": 5}}}}
	// //query
	// change := mgo.Change{
	// 	Update:    bson.M{"$set": &inInterface}, //bson.M{"$pull": bson.M{"metas": bson.M{"id": "metaid2" /*, "type": bson.M{"$ne": 5}*/}}, "$currentDate": bson.M{"lastmodified": true}},
	// 	Upsert:    true,
	// 	ReturnNew: true,
	// 	Remove:    false,
	// }
	// var foo interface{}
	// _ = mgoSession.DB("main").C("chats").Find(selector).One(&foo)
	// if err != nil {
	// 	fmt.Println("errselect: ", err)
	// }
	// fmt.Println("foo: ", foo)
	// //foo = nil
	// _, err = mgoSession.DB("main").C("chats").Find(selector).Apply(change, nil)
	// if err != nil {
	// 	fmt.Println("errupdate: ", err)
	// }
	// fmt.Println("foo: ", foo)
	// emailVerifyInfo := make(map[string]string, 2)

	// fmt.Println("uuid: ", len(emailVerifyInfo))

	// var n int = 12345
	// s := strconv.Itoa(n)
	// ss, er := strconv.ParseInt(s, 10, 16)
	// fmt.Println("num: ", ss, er, len(s))

}

// // check root meta ?????
// query := &bson.M{"_id": froot, "deleted": bson.M{"$exists": false}, "$or": []bson.M{{"metas": &meta{Type: 0, Id: metaid}}, {"metas": &meta{Type: 1, Id: metaid}}}}
// var foo interface{}

// err = conf.mgoColl.Find(query).One(&foo)
// if err != nil {
// 	if err == mgo.ErrNotFound {
// 		return suckhttp.NewResponse(403, "Forbidden"), nil
// 	}
// 	return nil, err
// }
// //
