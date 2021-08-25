package getauth

import (
	"bytes"
	"context"
	"io"
	"os"
	"time"
)

type GetAuthConfig struct {
	filePath  string
	keyLen    int
	valueLen  int
	rules     map[string][]byte
	someError chan error
}

func InitGetAuthorizer(ctx context.Context, filepath string, keylen int, valuelen int, warmingticktime time.Duration, backupticktime time.Duration) *GetAuthConfig {

	conf := &GetAuthConfig{filePath: filepath, keyLen: keylen, valueLen: valuelen, rules: make(map[string][]byte), someError: make(chan error, 1)}

	go conf.warmUp(ctx, warmingticktime)
	return conf
}

// жирновато?
func (c *GetAuthConfig) Check(key string, value []byte) bool {
	return bytes.Equal(c.rules[key], value)
}

func (c *GetAuthConfig) warmUp(ctx context.Context, ticktime time.Duration) {
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
