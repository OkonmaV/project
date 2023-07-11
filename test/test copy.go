package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

func cnv(s string) {
	// the string we want to transform
	fmt.Println(s)

	// --- Encoding: convert s from UTF-8 to ShiftJIS
	// declare a bytes.Buffer b and an encoder which will write into this buffer
	var b bytes.Buffer
	wInUTF8 := transform.NewWriter(&b, charmap.Windows1251.NewDecoder().Transformer)
	// encode our string
	wInUTF8.Write([]byte(s))
	wInUTF8.Close()
	// print the encoded bytes
	fmt.Printf("%#v\n", b)
	encS := b.String()
	fmt.Println(encS)

	// --- Decoding: convert encS from ShiftJIS to UTF8
	// declare a decoder which reads from the string we have just encoded
	rInUTF8 := transform.NewReader(strings.NewReader(encS), charmap.Windows1251.NewDecoder())
	// decode our string
	decBytes, _ := ioutil.ReadAll(rInUTF8)
	decS := string(decBytes)
	fmt.Println(decS)
}
