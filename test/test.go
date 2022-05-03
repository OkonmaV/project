package main

import (
	"context"
	"encoding/json"
	"time"

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
	ID   string `json:"id"`
	Text string `json:"text"`
}

func main() {
	msg := &message{ID: "client1", Text: "oh hi mark"}

	jmsg, err := json.Marshal(msg)
	if err != nil {
		println(err.Error())
		return
	}

	conn, _, _, err := ws.Dial(context.Background(), "ws://127.0.0.1:9010/cl1")
	if err != nil {
		println(err.Error())
		return
	}
	err = wsutil.WriteClientBinary(conn, jmsg)
	if err != nil {
		println(err.Error())
		return
	}
	jmsg1 := make([]byte, 5)
	jmsg2 := make([]byte, len(jmsg)-5)
	copy(jmsg1, jmsg[:5])
	copy(jmsg2, jmsg[5:])
	println("WRITED")
	err = wsutil.WriteClientBinary(conn, jmsg)
	if err != nil {
		println(err.Error())
		return
	}
	time.Sleep(time.Second * 2)
	fr1 := ws.NewBinaryFrame(jmsg)

	// r = wsutil.NewReader(conn, ws.StateServerSide).NextFrame()
	//fr1.Header.Fin = false
	// fr2 := ws.NewBinaryFrame(jmsg2)
	// fr2.Header.OpCode = ws.OpContinuation
	ws.MaskFrameInPlace(fr1)
	// ws.MaskFrameInPlace(fr2)

	time.Sleep(time.Second * 5)
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
