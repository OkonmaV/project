package repoTarantool

import (
	"time"

	"github.com/tarantool/go-tarantool"
)

func ConnectToTarantool(trntlAddr string) (*tarantool.Connection, error) {
	return tarantool.Connect(trntlAddr, tarantool.Opts{
		// User: ,
		// Pass: ,
		Timeout:       500 * time.Millisecond,
		Reconnect:     1 * time.Second,
		MaxReconnects: 4,
	})
}
