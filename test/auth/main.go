package main

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type config struct {
	DatafilePath string
	KeyLen       int
	ValueLen     int
}

func main() {
	var conf config
	if _, err := toml.DecodeFile("config.toml", &conf); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(conf)

}

func warmUp(keylen int, valuelen int, data map[string]string) error {

}
