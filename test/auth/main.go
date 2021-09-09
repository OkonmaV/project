package main

import (
	"context"
	"fmt"

	"project/test/auth/getauth"
	"project/test/auth/logscontainer"
	"project/test/auth/logscontainer/flushers"
	"project/test/auth/setauth"
	"time"

	"github.com/BurntSushi/toml"
)

type config struct {
	AuthFilePath string
	AuthKeyLen   int
	AuthValueLen int
}

func readTomlConfig(path string, conf *config) error {
	if _, err := toml.DecodeFile(path, conf); err != nil {
		return err
	}
	//fmt.Println("config: ", conf)
	return nil
}

func main() {
	conf := &config{}
	if err := readTomlConfig("config.toml", conf); err != nil {
		fmt.Println(err)
		return
	}
	ctx := context.Background()
	l, err := logscontainer.NewLogsContainer(ctx, flushers.NewConsoleFlusher("ohhimark"), 3, time.Second, 2)
	if err != nil {
		fmt.Println(err)
		return
	}

	getAuth, err := getauth.InitGetAuthorizer(ctx, conf.AuthFilePath, conf.AuthKeyLen, conf.AuthValueLen, time.Second*2, l)
	setAuth := setauth.InitSetAuthorizer(ctx, conf.AuthFilePath, conf.AuthKeyLen, conf.AuthValueLen)
	//
	if err != nil {
		fmt.Println(err)
		return
	}
	if err = setAuth.SetRule(ctx, "AAA", []byte("1")); err != nil {
		l.Error("SetRule", err)

	}
	if err = setAuth.SetRule(ctx, "BBB", []byte("1")); err != nil {
		l.Error("SetRule", err)
	}
	time.Sleep(time.Second * 3)
	fmt.Println("AAA true:", getAuth.Check("AAA", []byte("1")), "BBB false:", getAuth.Check("BBB", []byte("2")))
	//
	if err = setAuth.SetRule(ctx, "AAA", []byte("2")); err != nil {
		l.Error("SetRule", err)
	}
	time.Sleep(time.Second * 3)
	fmt.Println("AAA edited true:", getAuth.Check("AAA", []byte("2")))
	// checking errors
	if err = setAuth.SetRule(ctx, "BBB", []byte("111")); err != nil {
		l.Error("SetRule", err)
	}
	if err = setAuth.SetRule(ctx, "BBBB", []byte("1")); err != nil {
		l.Error("SetRule", err)
	}
	if err = setAuth.SetRule(ctx, "", []byte("1")); err != nil {
		l.Error("SetRule", err)
	}
	if err = setAuth.SetRule(ctx, "BBB", []byte("")); err != nil {
		l.Error("SetRule", err)
	}
	fmt.Println("sleep")
	time.Sleep(time.Second * 10)
}
