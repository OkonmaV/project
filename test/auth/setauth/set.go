package setauth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/big-larry/suckutils"
)

type SetAuthConfig struct {
	filePath  string
	keyLen    int
	valueLen  int
	rules     map[string][]byte
	someError chan error
}

func InitSetAuthorizer(ctx context.Context, filepath string, keylen int, valuelen int, warmingticktime time.Duration, backupticktime time.Duration) *SetAuthConfig {

	conf := &SetAuthConfig{filePath: filepath, keyLen: keylen, valueLen: valuelen, rules: make(map[string][]byte), someError: make(chan error, 1)}

	go conf.warmUp(ctx, warmingticktime)
	return conf
}

// жирновато?
func (c *SetAuthConfig) Check(key string, value []byte) bool {
	return bytes.Equal(c.rules[key], value)
}

func (c *SetAuthConfig) SetRule(ctx context.Context, key string, value []byte) error {

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

func (c *SetAuthConfig) warmUp(ctx context.Context, ticktime time.Duration) {
	ctx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(ticktime)
	rulelen := c.keyLen + c.valueLen
	var lastmodified time.Time
	var lastsize int64

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
			}
		}
	}

}
