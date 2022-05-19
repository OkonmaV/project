package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"project/services/messages/messagestypes"
	"project/wsconnector"
	"strconv"
	"strings"
	"time"

	"github.com/big-larry/mgo/bson"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// type Category struct {
// 	ID   int32
// 	Name string
// 	Slug string
// }

// type Post struct {
// 	ID         int32
// 	Categories []Category
// 	Title      string
// 	Slug       string
// 	Text       string
// }

// type cacheable interface {
// 	Category | Post
// }
// type cache[T cacheable] struct {
// 	data map[string]T
// }

// func New[T cacheable]() *cache[T] {
// 	c := &cache[T]{}
// 	c.data = make(map[string]T)

// 	return c
// }

// func (c *cache[T]) Set(key string, value T) {
// 	c.data[key] = value
// }

// func (c *cache[T]) Get(key string) (v T) {
// 	if v, ok := c.data[key]; ok {
// 		return v
// 	}

// 	return
// }

type message struct {
	UserId  string                           `json:"userid"`
	ChatId  string                           `json:"chatid"`
	Type    messagestypes.MessageContentType `json:"mtype"`
	ErrCode int                              `json:"type"`
	Data    []byte                           `json:"data"`
	Time    time.Time                        `json:"time"`
}

func readmessage(conn net.Conn) (*message, error) {
	h, r, err := wsutil.NextReader(conn, ws.StateClientSide)
	if err != nil {
		return nil, err
	}
	if h.OpCode.IsControl() {
		return nil, errors.New("control frame")
	}
	m := &message{}
	wsconnector.ReadAndDecodeJson(r, m)
	return m, err
}

// image formats and magic numbers
var magicTable = map[string]string{
	"\xff\xd8\xff":      "image/jpeg",
	"\x89PNG\r\n\x1a\n": "image/png",
	"GIF87a":            "image/gif",
	"GIF89a":            "image/gif",
}

// mimeFromIncipit returns the mime type of an image file from its first few
// bytes or the empty string if the file does not look like a known file type
func mimeFromIncipit(incipit []byte) string {
	incipitStr := string(incipit)
	for magic, mime := range magicTable {
		if strings.HasPrefix(incipitStr, magic) {
			return mime
		}
	}

	return ""
}

func main() {
	unp := "0"
	h, err1 := bson.NewObjectIdFromHex("6285c4c7cfda2e47e28d470c")
	f, err := strconv.ParseUint(unp, 10, 1)
	fmt.Println(f, err, h, err1)
	return
	msg1 := &message{UserId: "user1", ChatId: "someeechat", Type: messagestypes.Text, Data: []byte("text 11111")}
	msg2 := &message{UserId: "user2", ChatId: "someeechat", Type: messagestypes.Image}

	jmsg1, err := json.Marshal(msg1)
	if err != nil {
		println(err.Error())
		return
	}
	file, err := os.ReadFile("testpics/testpic1.jpg")
	if err != nil {
		println(err.Error())
		return
	}

	println(mimeFromIncipit(file))
	msg2.Data = file
	jmsg2, err := json.Marshal(msg2)

	if err != nil {
		println(err.Error())
		return
	}
	// f := &message{}
	// json.Unmarshal(jmsg1, f)
	// fmt.Println(f)
	// return
	conn1, _, _, err := ws.Dial(context.Background(), "ws://127.0.0.1:8092/user1")
	if err != nil {
		println(err.Error())
		return
	}
	conn2, _, _, err := ws.Dial(context.Background(), "ws://127.0.0.1:8092/user2")
	if err != nil {
		println(err.Error())
		return
	}

	time.Sleep(time.Second)

	err = wsutil.WriteClientBinary(conn1, jmsg1)
	if err != nil {
		println(err.Error())
		return
	}

	// confirmed_msg, err := wsconnector.EncodeJson(msg1)
	// if err != nil {
	// 	println(err.Error())
	// 	return
	// }
	// f := &message{}
	// json.Unmarshal(confirmed_msg, f)
	// fmt.Println(f)
	// return

	err = wsutil.WriteClientBinary(conn2, jmsg2)
	if err != nil {
		println(err.Error())
		return
	}
	time.Sleep(time.Second)

	r1, err := readmessage(conn1)
	if err != nil {
		println(err.Error())
		return
	}
	fmt.Println("message to user1:", r1)
	r11, err := readmessage(conn1)
	if err != nil {
		println(err.Error())
		return
	}
	fmt.Println("message to user1:", r11)

	r2, err := readmessage(conn2)
	if err != nil {
		println(err.Error())
		return
	}
	fmt.Println("message to user2:", r2)
	r22, err := readmessage(conn2)
	if err != nil {
		println(err.Error())
		return
	}
	fmt.Println("message to user2:", r22)
	time.Sleep(time.Second * 2)

	// ws.Cipher(fr2.Payload, fr1.Header.Mask, 0)
	// bfr1, err := ws.CompileFrame(fr1)
	// if err != nil {
	// 	println(err.Error())
	// 	return
	// }
	// bfr2, err := ws.CompileFrame(fr2)
	// if err != nil {
	// 	println(err.Error())
	// 	return
	// }
	// b := bytes.Join([][]byte{bfr1, bfr2}, []byte{})
	// _, err = conn.Write(b)
	// if err != nil {
	// 	println(err.Error())
	// 	return
	// }
	// return
	// err = ws.WriteFrame(conn, fr1)
	// if err != nil {
	// 	println(err.Error())
	// 	return
	// }
	// err = ws.WriteFrame(conn, fr2)
	// if err != nil {
	// 	println(err.Error())
	// 	return
	// }
	// // create a new category
	// category := Category{
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
	// postcache.Set(post.Slug, post)
	// fmt.Println(postcache.Get("postslug1"))

}
