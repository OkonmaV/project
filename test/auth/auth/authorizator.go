package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"syscall"
	"time"

	"thin-peak/httpservice"

	"github.com/big-larry/suckutils"
)

type AuthConfig struct {
	filePath  string
	keyLen    int
	valueLen  int
	rules     map[string][]byte
	someError chan error
}

func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(int64(ts.Sec), int64(ts.Nsec))
}

func InitNewAuthorizer(ctx context.Context, filepath string, keylen int, valuelen int, warmingticktime time.Duration, backupticktime time.Duration) AuthConfig {

	conf := AuthConfig{filePath: filepath, keyLen: keylen, valueLen: valuelen, rules: make(map[string][]byte), someError: make(chan error, 1)}

	go warmUp(ctx, warmingticktime, conf)
	return conf
}

func InitNewBackup() {}

// жирновато?
func (c AuthConfig) Check(key string, value []byte) bool {
	return bytes.Compare(c.rules[key], value) == 0
}

func checkError(someError chan error) error {
	select {
	case err := <-someError:
		return err
	default:
		return nil
	}
}

func SetRule(ctx context.Context, key string, value []byte, c AuthConfig) error {

	file, err := suckutils.OpenConcurrentFile(ctx, c.filePath, time.Millisecond*100)
	if err != nil {
		return err
	}
	defer file.Close()

	data := make([]byte, c.keyLen+c.valueLen)

	if len(key) == c.keyLen {
		copy(data, []byte(key))
	} else {
		return errors.New("key len mismatch")
	}

	if len(value) == c.valueLen {
		copy(data[c.keyLen:], []byte(value))
	} else {
		return errors.New("value len mismatch")
	}

	fileinfo, err := file.File.Stat()
	if err != nil {
		return err
	}

	if _, err = file.File.Seek(fileinfo.Size()/int64(c.keyLen+c.valueLen)*int64(c.keyLen+c.valueLen), 0); err != nil {
		return err
	}
	if _, err = file.File.Write(data); err != nil {
		return err
	}
	return nil
}

func backup(ctx context.Context, filepath string, backupticktime time.Duration, thisservicename string, backupserver *httpservice.InnerService) {

}

func warmUp(ctx context.Context, ticktime time.Duration, c AuthConfig) {
	ctx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(ticktime)
	rulelen := c.keyLen + c.valueLen
	var lastmodified time.Time
	var lastsize int64
	lastday := time.Now().Day()

	file, err := os.OpenFile(c.filePath, os.O_CREATE|os.O_RDONLY, 0777)
	if err != nil {
		cancel()
		c.someError <- err
		return
	}

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			file.Close()
			//c.someError <- err
			cancel()
			return

		case <-ticker.C:
			fileinfo, err := file.Stat()
			if err != nil {
				cancel()
				c.someError <- err
				break
			}
			if fileinfo.ModTime().After(lastmodified) {
				lastmodified = fileinfo.ModTime()

				if lastsize < fileinfo.Size() {
					if _, err = file.Seek(lastsize, 0); err != nil {
						cancel()
						c.someError <- err
						break
					}

					filedata := make([]byte, fileinfo.Size()-lastsize)
					if _, err = file.Read(filedata); err != nil {
						c.someError <- err
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
						c.rules[string(rule[:c.keyLen])] = rule[c.keyLen:]
					}
					fileinfo.Sys()
				} else if lastday != time.Now().Day() { //} else if lastsize > fileinfo.Size() { // перечитываем файл полностью
					filedata := make([]byte, fileinfo.Size()/int64(rulelen)*int64(rulelen))
					if _, err = file.Read(filedata); err != nil {
						c.someError <- err
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
						c.rules[string(rule[:c.keyLen])] = rule[c.keyLen:]
					}
				}

			}
			//fmt.Println(time.Now()) //////////////////////////////////
		}
	}

}
