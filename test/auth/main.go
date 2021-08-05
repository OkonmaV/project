package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/big-larry/suckutils"
)

type config struct {
	FilePath string
	KeyLen   int
	ValueLen int
	Rules    map[string][]byte
}

func main() {
	//----
	var conf config
	if _, err := toml.DecodeFile("config.toml", &conf); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("config: ", conf)
	//----

	conf.Rules = make(map[string][]byte)
	ctx := context.Background()
	er := make(chan error)
	go warmUp(ctx, er, time.Second*3, conf)

	fmt.Println("wait for err...")
	fmt.Println("error: ", <-er)

}

func warmUp(ctxx context.Context, er chan error, timeout time.Duration, conf config) {
	ctx, cancel := context.WithCancel(ctxx)
	ticker := time.NewTicker(timeout)
	rulelen := conf.KeyLen + conf.ValueLen
	var lastmodified time.Time
	var lastsize int64

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
			er <- err
			cancel()
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

				if lastsize < fileinfo.Size() {
					if _, err = file.Seek(lastsize, 0); err != nil {
						cancel()
						er <- err
						break
					}

					filedata := make([]byte, fileinfo.Size()-lastsize)
					if _, err = file.Read(filedata); err != nil {
						er <- err
						cancel()
						break
					}

					r := bytes.NewReader(filedata)
					rule := make([]byte, rulelen)
					for {
						if _, err = r.Read(rule); err != nil {
							if err == io.EOF {
								break
							}
						}
						conf.Rules[string(rule[:conf.KeyLen])] = rule[conf.KeyLen:]
					}
					fileinfo.Sys()
				} else if lastsize > fileinfo.Size() { // перечитываем файл полностью
					filedata := make([]byte, fileinfo.Size()/int64(rulelen)*int64(rulelen))
					if _, err = file.Read(filedata); err != nil {
						er <- err
						cancel()
						break
					}

					r := bytes.NewReader(filedata)
					rule := make([]byte, rulelen)
					for {
						if _, err = r.Read(rule); err != nil {
							if err == io.EOF {
								break
							}
						}
						conf.Rules[string(rule[:conf.KeyLen])] = rule[conf.KeyLen:]
					}
				}
			}
		}
	}

}

func setRule(ctx context.Context, key string, value []byte, conf config) error {

	file, err := suckutils.OpenConcurrentFile(ctx, conf.FilePath, time.Millisecond*100)
	if err != nil {
		return err
	}
	defer file.Close()

	data := make([]byte, conf.KeyLen+conf.ValueLen)

	if len(key) == conf.KeyLen {
		copy(data, []byte(key))
	} else {
		return errors.New("key len mismatch")
	}

	if len(value) == conf.ValueLen {
		copy(data[conf.KeyLen:], []byte(value))
	} else {
		return errors.New("value len mismatch")
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
