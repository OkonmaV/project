package setauth

import (
	"context"
	"errors"
	"time"

	"github.com/big-larry/suckutils"
)

type SetAuthConfig struct {
	filePath string
	keyLen   int
	valueLen int
	rules    map[string][]byte
}

func InitSetAuthorizer(ctx context.Context, filepath string, keylen int, valuelen int) *SetAuthConfig {

	conf := &SetAuthConfig{filePath: filepath, keyLen: keylen, valueLen: valuelen, rules: make(map[string][]byte)}

	return conf
}

func (c *SetAuthConfig) SetRule(ctx context.Context, key string, value []byte) error {
	// DO NOT USE O_APPEND
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

	// запись поверх кривого правила
	var offsetCorrection int64 = 0
	if fileinfo.Size()%int64(c.keyLen+c.valueLen) != 0 {
		// запись после кривого правила
		offsetCorrection = int64(c.keyLen + c.valueLen)
	}
	if _, err = file.File.Seek(fileinfo.Size()/int64(c.keyLen+c.valueLen)*int64(c.keyLen+c.valueLen)+offsetCorrection, 0); err != nil {
		return err
	}

	if _, err = file.File.Write(data); err != nil {
		return err
	}
	return nil
}
