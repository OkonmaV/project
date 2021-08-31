package setauth

import (
	"context"
	"errors"
	"os"
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
	file, err := OpenConcurrentFile(ctx, c.filePath, time.Millisecond*100)
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

// without o_append
type ConcurrentFile struct {
	File         *os.File
	lockFilename string
}

func OpenConcurrentFile(ctx context.Context, filename string, timeout time.Duration) (*ConcurrentFile, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	timer := time.NewTicker(time.Millisecond * 100)
	lockFilename := suckutils.ConcatTwo(filename, ".lock")
	var err error
	var result *ConcurrentFile
	for {
		select {
		case <-ctx.Done():
			timer.Stop()
			if result == nil && err == nil {
				err = errors.New("Timeout")
			}
			cancel()
			return result, err
		case <-timer.C:
			f, err := os.OpenFile(lockFilename, os.O_CREATE|os.O_EXCL, 0664)
			if e, ok := err.(*os.PathError); ok && errors.Is(e.Err, os.ErrExist) {
				break
			} else if err != nil {
				cancel()
				break
			}
			f.Close()
			f, err = os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0664)
			if err == nil {
				result = &ConcurrentFile{File: f, lockFilename: lockFilename}
			}
			cancel()
		}
	}
}

func (file *ConcurrentFile) Close() error {
	err := file.File.Close()
	if err != nil {
		return err
	}
	return os.Remove(file.lockFilename)
}
