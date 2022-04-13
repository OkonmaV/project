package main

import (
	"fmt"
	"mime"
)

type test1 struct {
	DataStr      string
	DataStrSlice []string
	DataStruct   *test2
}
type test2 struct {
	DataInt      int
	DataIntSlice []int
}

func main() {
	// t := &test1{}
	// tt := &test2{}
	// if err := confdecoder.DecodeFile("config.txt", t, tt); err != nil {
	// 	println(err.Error())
	// } else {
	// 	fmt.Println(t, tt)
	// }
	f, s, err := mime.ParseMediaType("")

	fmt.Println(f, s, err)
}
