package getauth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"project/test/logscontainer"

	"time"
)

type GetAuthConfig struct {
	filePath string
	keyLen   int
	valueLen int
	rules    map[string][]byte
}

func InitGetAuthorizer(ctx context.Context, filepath string, keylen int, valuelen int, warmingticktime time.Duration, l *logscontainer.LogsContainer) (*GetAuthConfig, error) {
	if warmingticktime == 0 {
		return nil, errors.New("ticktime must be greater than 0")
	}
	conf := &GetAuthConfig{filePath: filepath, keyLen: keylen, valueLen: valuelen, rules: make(map[string][]byte)}

	go conf.warmUp(ctx, warmingticktime, l)
	return conf, nil
}

func (c *GetAuthConfig) Check(key string, value []byte) bool {
	return bytes.Equal(c.rules[key], value)
}

func (c *GetAuthConfig) warmUp(ctx context.Context, ticktime time.Duration, l *logscontainer.LogsContainer) {
	ctx, cancel := context.WithCancel(ctx) //
	ticker := time.NewTicker(ticktime)
	rulelen := c.keyLen + c.valueLen
	var lastmodified time.Time
	var lastsize int64

	file, err := os.OpenFile(c.filePath, os.O_CREATE|os.O_RDONLY, 0777)
	if err != nil {
		l.Error("OpenFile", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			file.Close()
			return

		case <-ticker.C:
			fileinfo, err := file.Stat()
			fmt.Println("FS:", fileinfo) //
			if err != nil {
				l.Error("FileStat", err)
				cancel()
				break
			}
			if fileinfo.ModTime().After(lastmodified) {
				lastmodified = fileinfo.ModTime()

				if _, err = file.Seek(lastsize, 0); err != nil {
					l.Error("FileSeek", err)
					cancel()
					break
				}

				filedata := make([]byte, fileinfo.Size()-lastsize)
				if _, err = file.Read(filedata); err != nil {
					l.Error("FileRead", err)
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
