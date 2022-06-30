package main

import (
	"confdecoder"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"
)

type Category struct {
	ID   int32
	Name string
	Slug string
}

type Post struct {
	ID         int32
	Categories []Category
	Title      string
	Slug       string
	Text       string
}

type cacheable interface {
	Category | Post
}
type cache[T cacheable] struct {
	data map[string]T
}

func New[T cacheable]() *cache[T] {
	c := &cache[T]{}
	c.data = make(map[string]T)

	return c
}

func (c *cache[T]) Set(key string, value T) {
	c.data[key] = value
}

func (c *cache[T]) Get(key string) (v T) {
	if v, ok := c.data[key]; ok {
		return v
	}

	return
}

// type message struct {
// 	UserId  string                           `json:"userid"`
// 	ChatId  string                           `json:"chatid"`
// 	Type    messagestypes.MessageContentType `json:"mtype"`
// 	ErrCode int                              `json:"type"`
// 	Data    []byte                           `json:"data"`
// 	Time    time.Time                        `json:"time"`
// }

// func readmessage(conn net.Conn) (*message, error) {
// 	h, r, err := wsutil.NextReader(conn, ws.StateClientSide)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if h.OpCode.IsControl() {
// 		return nil, errors.New("control frame")
// 	}
// 	m := &message{}
// 	wsconnector.ReadAndDecodeJson(r, m)
// 	return m, err
// }

// // image formats and magic numbers
// var magicTable = map[string]string{
// 	"\xff\xd8\xff":      "image/jpeg",
// 	"\x89PNG\r\n\x1a\n": "image/png",
// 	"GIF87a":            "image/gif",
// 	"GIF89a":            "image/gif",
// }

// // mimeFromIncipit returns the mime type of an image file from its first few
// // bytes or the empty string if the file does not look like a known file type
// func mimeFromIncipit(incipit []byte) string {
// 	incipitStr := string(incipit)
// 	for magic, mime := range magicTable {
// 		if strings.HasPrefix(incipitStr, magic) {
// 			return mime
// 		}
// 	}

// 	return ""
// }
// func GetMD5(str string) (string, error) {
// 	hash := md5.New()
// 	_, err := hash.Write([]byte(str))
// 	if err != nil {
// 		return "", err
// 	}
// 	return hex.EncodeToString(hash.Sum(nil)), nil
// }

func readLoop(conn net.Conn, side_liter string, startreading chan struct{}) {
	fmt.Println("side", side_liter, " readloop started")
	for {
		buf := make([]byte, 20)
		startreading <- struct{}{}
		fmt.Println("side", side_liter, " reading")
		n, err := conn.Read(buf)
		fmt.Println("side", side_liter, " readed")
		fmt.Println("side", side_liter, " readed | err:", err, "| n:", n, "| buf:", buf)

	}
}

func acceptLoop(ln net.Listener, side_liter string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("side", side_liter, "| accept | err:", err)
			continue
		}
		fmt.Println("side", side_liter, "| accept | from:", conn.RemoteAddr().String())
		ch := make(chan struct{}, 1)
		go readLoop(conn, side_liter, ch)
		go writeAfterTimeout(conn, side_liter, ch)
	}
}

func writeAfterTimeout(conn net.Conn, side_liter string, startreading chan struct{}) {
	msg := []byte{1, 1}

	<-startreading
	//time.Sleep(time.Second)

	fmt.Println("side", side_liter, "writing")
	n, err := conn.Write(msg)
	fmt.Println("side", side_liter, "written:", msg, "| n:", n, "| err:", err)
}

type ssdfg struct {
	Uid string
	Num int
}

type ht string

func main() {
	buf := []byte{7, 5, 5, 5}
	buf2 := []byte{0, 5, 5, 5}
	buf3 := []byte{7, 0, 0, 0}
	fmt.Println(binary.BigEndian.Uint32(buf2), binary.BigEndian.Uint32(buf))
	fmt.Println(binary.BigEndian.Uint32(buf) & 16777215)
	fmt.Println(binary.BigEndian.Uint32(buf3)>>24, binary.BigEndian.Uint32(buf)>>24)
	return

	// connuid := uint32(50)
	// appid := uint16(15)
	// headers := []byte{21, 22, 23, 24, 25}
	// body := []byte{31, 32, 33, 34, 35}
	// client_encoded_message, err1 := protocol.EncodeClientMessage(protocol.TypeText, protocol.AppID(appid), headers, body)
	// client_decoded_message, err2 := protocol.DecodeClientMessage(client_encoded_message)
	// fmt.Println(1, client_decoded_message, err1, err2)
	// appserv_decoded_message, err3 := protocol.DecodeClientMessageToAppServerMessage(client_encoded_message)
	// fmt.Println(2, appserv_decoded_message, err3, client_encoded_message)
	// appservclient_encoded_message := appserv_decoded_message.EncodeToClientMessage()
	// clientappserv_decoded_message, err4 := protocol.DecodeClientMessage(appservclient_encoded_message)
	// fmt.Println(3, clientappserv_decoded_message, err4, appservclient_encoded_message)
	// appserv_decoded_message.ConnectionUID = protocol.ConnUID(connuid)
	// appservapp_encoded_message, _ := appserv_decoded_message.EncodeToAppMessage()
	// appappserv_decoded_message, err5 := protocol.DecodeAppMessage(appservapp_encoded_message)
	// fmt.Println(4, appappserv_decoded_message, err5)
	// appservapp_encoded_message2, err6 := appappserv_decoded_message.Encode()
	// appappserv_decoded_message2, err7 := protocol.DecodeAppMessage(appservapp_encoded_message2)
	// fmt.Println(5, appappserv_decoded_message2, err6, err7)

	return
	pfd, err := confdecoder.ParseFile("config.txt")
	if err != nil {
		panic("parsing config.txt err: " + err.Error())
	}
	strr := &struct{ DataIntt int }{}
	if err := pfd.DecodeTo(strr); err != nil {
		panic("decoding config.txt err: " + err.Error())
	}
	fmt.Println(strr.DataIntt)
	return
	t := ssdfg{Uid: "smth", Num: 5}
	tt := ssdfg{Uid: "tsm", Num: 4}
	//fgh := ht("ht")
	jt, err := json.Marshal(&[]interface{}{t, tt})
	if err != nil {
		println(err.Error())
		return
	}
	m := make([]interface{}, 5)

	err = json.Unmarshal(jt, &m)
	if err != nil {
		println(err.Error())
		return
	}
	rand.Seed(time.Now().UnixMicro())
	fmt.Println(m, "|||||", strings.Replace(fmt.Sprint(rand.Perm(6)), " ", "", -1))
	return
	a_ln, err := net.Listen("tcp", "127.0.0.1:9091")
	if err != nil {
		println("A side listen err:", err.Error())
		return
	}
	go acceptLoop(a_ln, "A")
	// b_ln, err := net.Listen("tcp", "127.0.0.1:9092")
	// if err != nil {
	// 	println("B side listen err:", err.Error())
	// 	return
	// }
	//go acceptLoop(b_ln, "B")

	// a_to_b_conn, err := net.Dial("tcp", "127.0.0.1:9092")
	// if err != nil {
	// 	println("A to B dial err:", err.Error())
	// 	return
	// }
	//time.Sleep(time.Second * 2)
	b_to_a_conn, err := net.Dial("tcp", "127.0.0.1:9091")
	if err != nil {
		println("B to A dial err:", err.Error())
		return
	}
	ch := make(chan struct{}, 1)
	go readLoop(b_to_a_conn, "B", ch)
	<-ch

	time.Sleep(time.Second * 2)

	// ab := []byte{1, 1}
	// fmt.Println("A to B writting")
	// na, erra := a_to_b_conn.Write(ab)
	// fmt.Println("A to B written:", ab, "| n:", na, "| err:", erra)

	// time.Sleep(time.Second * 2)

	// bb := []byte{2, 2}
	// fmt.Println("B to A writting")
	// nb, errb := b_to_a_conn.Write(bb)
	// fmt.Println("B to A written:", bb, "| n:", nb, "| err:", errb)

	time.Sleep(time.Second * 2)
	return
	// flusher := logger.NewFlusher(encode.DebugLevel)
	// l := flusher.NewLogsContainer("testtag1", "testtag2")
	// l.Debug("Hey", "Debug")
	// l.Info("Hey", "Info")
	// l.Warning("Hey", "Warning")
	// l.Error("Hey", errors.New("error"))
	// flusher.Close()
	// <-flusher.Done()
	// //time.Sleep(time.Second * 5)
	// return
	// create a new category
	category := Category{
		ID:   1,
		Name: "Cat1",
		Slug: "catslug",
	}
	// create cache for Category struct
	catcache := New[Category]()
	// add category to cache
	catcache.Set(category.Slug, category)

	// create a new post
	post := Post{
		ID: 1,
		Categories: []Category{
			{ID: 1, Name: "Cat1", Slug: "catslug"},
		},
		Title: "posttitle",
		Text:  "posttext",
		Slug:  "postslug",
	}
	// create cache for Post struct
	postcache := New[Post]()
	// add post to cache
	postcache.Set(post.Slug, post)
	// g := `category := Category{
	// 	ID:   1,
	// 	Name: "Cat1",
	// 	Slug: "catslug",
	// }
	// // create cache for Category struct
	// catcache := New[Category]()
	// // add category to cache
	// catcache.Set(category.Slug, category)

	// // create a new post
	// post := Post{
	// 	ID: 1,
	// 	Categories: []Category{
	// 		{ID: 1, Name: "Cat1", Slug: "catslug"},
	// 	},
	// 	Title: "posttitle",
	// 	Text:  "posttext",
	// 	Slug:  "postslug",
	// }
	// // create cache for Post struct
	// postcache := New[Post]()
	// // add post to cache
	// postcache.Set(post.Slug, post)`

}
