package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"project/logs/encode"
	"project/logs/logger"
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
type foo []byte

type str struct {
	One int
	Two int
	Int interface{}
}
type ins struct {
	Three int
}

func (f foo) Read(b []byte) (int, error) {
	bb, err := json.Marshal(str{One: 1, Two: 2, Int: ins{Three: 3}})
	if err != nil {
		println("this")
		return 0, err
	}
	copy(b, bb)
	//b = append(b[0:], bb...)
	return len(bb), nil
}

func main() {
	var f foo
	m := &str{}
	err := json.NewDecoder(f).Decode(m)
	fmt.Println(m, err)
	return
	flusher := logger.NewFlusher(encode.DebugLevel)
	l := flusher.NewLogsContainer("testtag1", "testtag2")
	l.Debug("Hey", "Debug")
	l.Info("Hey", "Info")
	l.Warning("Hey", "Warning")
	l.Error("Hey", errors.New("error"))
	flusher.Close()
	<-flusher.Done()
	//time.Sleep(time.Second * 5)
	return
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
	g := `category := Category{
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
	postcache.Set(post.Slug, post)`
	jj := 65535
	buf := []byte{0, 0, 9, 9, 9}
	binary.LittleEndian.PutUint16(buf[1:], uint16(jj))
	var bb []byte
	fmt.Println(postcache.Get("postslug1"), len(g), []byte("["), []byte("]"), []byte(" "), buf, append(buf, bb...), append(buf, ""...), len(buf))

}
