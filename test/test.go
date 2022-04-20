package main

import (
	"fmt"
)

type test1 struct {
	DataStr    string
	DataStruct *test2
}
type test2 struct {
	DataInt int
}

func main() {
	// t := &test1{}
	// tt := &test2{}
	// if err := confdecoder.DecodeFile("config.txt", t, tt); err != nil {
	// 	println(err.Error())
	// } else {
	// 	fmt.Println(t, tt)
	// }
	f := test1{}
	f1 := test1{}
	fmt.Println(f == f1)
}
