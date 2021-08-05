package main

import (
	"fmt"
	"lib"
	"time"
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
	Meta string
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
type quiz struct {
	Questions map[string]question
}
type question struct {
	Type     int      `bson:"question_type" json:"question_type"`
	TypeTag  string   `bson:"-" json:"-"`
	Position int      `bson:"question_position" json:"question_position"`
	Text     string   `bson:"question_text" json:"question_text"`
	Answers  []answer `bson:"answers" json:"answers"`
}
type answer struct {
	Id   string `bson:"answer_id" json:"id"`
	Text string `bson:"answer_text" json:"text"`
}

type auth struct {
	login    string
	password string
}

func main() {

	// trntlConn, err := tarantool.Connect("127.0.0.1:3301", tarantool.Opts{
	// 	// User: ,
	// 	// Pass: ,
	// 	Timeout:       500 * time.Millisecond,
	// 	Reconnect:     1 * time.Second,
	// 	MaxReconnects: 4,
	// })
	// fmt.Println("errConn: ", err)
	// //foo := auth{"login2", "password2"}
	// err = trntlConn.UpsertAsync("auth", []interface{}{"login", "password"}, []interface{}{[]interface{}{"=", "password", "password"}}).Err()
	// fmt.Println("errUpsert: ", err)

	// var trntlAuthRec repo.TarantoolAuthTuple
	// err = trntlConn.SelectTyped("auth", "secondary", 0, 1, tarantool.IterEq, []interface{}{"login", "password"}, &trntlAuthRec)
	// fmt.Println("errSelect: ", err)
	// fmt.Println("trntlAuthRec: ", trntlAuthRec)

	// //ertrt := &tarantool.Error{Msg: suckutils.ConcatThree("Duplicate key exists in unique index 'primary' in space '", "regcodes", "'"), Code: tarantool.ErrTupleFound}

	//err := trntlConn.UpsertAsync("regcodes", []interface{}{28258, "123", "asd", "asd"}, []interface{}{[]interface{}{"=", "metaid", "NEWMETAID1"}}).Err()
	//fmt.Println("errUpsert:", err)

	//var trntlRes tuple
	//err = trntlConn.UpsertAsync("auth", []interface{}{"login", "password"}, []interface{}{[]interface{}{"=", "password", "password"}}).Err()
	//err = trntlConn.UpdateAsync("regcodes", "primary", []interface{}{28258}, []interface{}{[]interface{}{"=", "metaid", "metaid"}}).Err()
	//trntlConn.GetTyped("regcodes", "primary", []interface{}{28258}, &trntlRes)
	//fmt.Println("err:", err)
	//fmt.Println("resTrntl:", trntlRes)
	//fmt.Println()

	//err = trntlConn.SelectTyped("regcodes", "primary", 0, 1, tarantool.IterEq, []interface{}{28258}, &trntlRes)
	// //_, err = trntlConn.Update("regcodes", "primary", []interface{}{28258}, []interface{}{[]interface{}{"=", "metaid", "h"}, []interface{}{"=", "metaname", "hh"}})

	// mgoSession, err := mgo.Dial("127.0.0.1")
	// if err != nil {
	// 	return
	// }
	// mgoColl := mgoSession.DB("test").C("test")

	// ch, err := mgoColl.Upsert(bson.M{"field": 750}, bson.M{"$set": bson.M{"fi": 100}, "$setOnInsert": bson.M{"field": 750}})
	// fmt.Println("errinsert: ", err)

	// fmt.Println("err: ", err, ch.Matched, ch.Updated)
	// change := mgo.Change{
	// 	Upsert:    false,
	// 	Remove:    false,
	// 	ReturnNew: true,
	// 	Update:    bson.M{"$addToSet": bson.M{"fis": 2100}, "$setOnInsert": bson.M{"field": 1750}},
	// }
	// var res interface{}
	// ch, err = mgoColl.Find(bson.M{"field": 1750}).Apply(change, &res)
	// fmt.Println("errFindAndModify: ", err, ch.UpsertedId, "res:", res)
	b := make([]byte, 5)
	copy(b[2:], []byte("a"))
	fmt.Println(b, 6/2*10)

	//query2 := bson.M{"type": 1, "users": bson.M{"$all": []bson.M{{"$elemMatch": bson.M{"userid": "withUserId"}}, {"$elemMatch": bson.M{"userid": "userId"}}}}}
	//query2 := bson.M{"type": 1, "$or": []bson.M{{"users.0.userid": "withUserId", "users.1.userid": "userId"}, {"users.0.userid": "userId", "users.1.userid": "withUserId"}}}
	//bson.M{"$elemMatch": bson.M{"userid": "userId", "type": bson.M{"$ne": 1}}}}

	//err = mgoColl.Find(query2).Select(bson.M{"users.$": 1}).One(&mgores)
	//bar[1] = 1
	//bar[2] = 2

	// var foo answer = answer{Id: "id", Text: "text"}
	// answrs := make(map[string]*answer)
	// answrs["id"] = &foo
	// *answrs["id"] = answer{Text: "newtext"}
	// fmt.Println(foo)

	// ans1 := []answer{}
	// ans2 := []answer{}
	// ans1 = append(ans1, answer{Id: "aid1", Text: "ANS1TEXT"}, answer{Id: "aid11", Text: "ANS11TEXT"})
	// ans2 = append(ans2, answer{Id: "aid2", Text: "ANS2TEXT"}, answer{Id: "aid22", Text: "ANS22TEXT"})

	// holo := make(map[string]question)
	// holo["qid1"] = question{Type: 1, Text: "SOMETEXT1", Answers: ans1}
	// holo["qid2"] = question{Type: 2, Text: "SOMETEXT2", Answers: ans2}
	// holo["qid3"] = question{Type: 3, Text: "SOMETEXT111"}

	// templData, err := ioutil.ReadFile("index.html")
	// if err != nil {
	// 	fmt.Println("templerr1:", err)
	// 	return
	// }

	// templ, err := template.New("index").Parse(string(templData))
	// if err != nil {
	// 	fmt.Println("templerr2:", err)
	// 	return
	// }
	// var body []byte
	// buf := bytes.NewBuffer(body)
	// err = templ.Execute(buf, holo)
	// if err != nil {
	// 	fmt.Println("templerr3:", err)
	// 	return
	// }

	// fd := buf.String()

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
