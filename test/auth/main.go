package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/big-larry/suckutils"
)

type config struct {
	FilePath string
	KeyLen   int
	ValueLen int
	Rules    map[string]byte
}

func main() {
	var conf config
	if _, err := toml.DecodeFile("config.toml", &conf); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("config: ", conf)

	conf.Rules = make(map[string]byte)

	er := make(chan error)
	go warmUp(context.Background(), er, time.Second*3, conf)
	//----
	go func() {
		for {
			time.Sleep(time.Second * 5)
			rand.Seed(time.Now().UnixNano())
			setRuleIntValue("ccc", rand.Intn(50), conf)
			setRuleIntValue("ggg", rand.Intn(50), conf)
		}

	}()
	//----
	fmt.Println("wait for err...")
	fmt.Println("error: ", <-er)

}

func warmUp(ctxx context.Context, er chan error, timeout time.Duration, conf config) { // err????
	ctx, cancel := context.WithCancel(ctxx)
	ticker := time.NewTicker(timeout)
	var lastmodified time.Time

	file, err := os.Open(conf.FilePath)
	if err != nil {
		cancel()
		er <- err
		return
	}

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			file.Close()
			cancel()
			er <- err
			return

		case <-ticker.C:
			fileinfo, err := file.Stat()
			if err != nil {
				cancel()
				er <- err
				break
			}
			if fileinfo.ModTime().After(lastmodified) {
				lastmodified = fileinfo.ModTime()
				filedata := make([]byte, fileinfo.Size()/int64(conf.KeyLen+conf.ValueLen)*int64(conf.KeyLen+conf.ValueLen)) //filedata:=make([]byte)
				if _, err = file.Read(filedata); err != nil {
					cancel()
					break
				}

				r := bytes.NewReader(filedata)
				rule := make([]byte, conf.KeyLen+conf.ValueLen)
				for {
					if _, err = r.Read(rule); err != nil {
						if err == io.EOF {
							break
						}
					}
					conf.Rules[string(rule[:conf.KeyLen])] = rule[len(rule)-1]
					//conf.Rules[string(rule[:conf.KeyLen])] = string(rule[conf.KeyLen:])
				}
				fmt.Println(conf.Rules)

			}
		}
	}

}

func setRuleIntValue(key string, value int, conf config) error {

	file, err := suckutils.OpenConcurrentFile(context.Background(), conf.FilePath, time.Millisecond*100)
	if err != nil {
		return err
	}
	defer file.Close()

	data := make([]byte, conf.KeyLen+conf.ValueLen)

	if len(key) == conf.KeyLen {
		copy(data, []byte(key))
	} else {
		return errors.New("fuck key len")
	}

	data[conf.KeyLen] = byte(value)

	fileinfo, err := file.File.Stat()
	if err != nil {
		return err
	}

	if _, err = file.File.Seek(fileinfo.Size()/int64(conf.KeyLen+conf.ValueLen)*int64(conf.KeyLen+conf.ValueLen), 0); err != nil {
		return err
	}
	if _, err = file.File.Write(data); err != nil {
		return err
	}
	return nil
}

func setRuleStrValue(key string, value string, conf config) error {

	file, err := suckutils.OpenConcurrentFile(context.Background(), conf.FilePath, time.Millisecond*100)
	if err != nil {
		return err
	}
	defer file.Close()

	data := make([]byte, conf.KeyLen+conf.ValueLen)

	if len(key) == conf.KeyLen {
		copy(data, []byte(key))
	} else {
		return errors.New("fuck key len")
	}
	if len(value) == conf.ValueLen {
		copy(data[conf.KeyLen:], []byte(value))
	} else {
		return errors.New("fuck value len")
	}

	fileinfo, err := file.File.Stat()
	if err != nil {
		return err
	}

	if _, err = file.File.Seek(fileinfo.Size()/int64(conf.KeyLen+conf.ValueLen)*int64(conf.KeyLen+conf.ValueLen), 0); err != nil {
		return err
	}
	if _, err = file.File.Write(data); err != nil {
		return err
	}
	return nil
}
