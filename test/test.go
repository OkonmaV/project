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

	tt := []int{0, 1, 2, 3}
	tt = append(tt[:2], tt[2+1:]...)
	fmt.Println(tt, len(tt), cap(tt))
}
