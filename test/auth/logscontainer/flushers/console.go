package flushers

import (
	"fmt"
	"os"
	"project/test/auth/logscontainer"
)

type Console struct {
	name string
}

func NewConsoleFlusher(name string) *Console {
	return &Console{name: name}
}

func (c *Console) Flush(logs []logscontainer.Log) error {
	for i := 0; i < len(logs); i++ {
		fmt.Fprintf(os.Stdout, "[%s] [%s] [%s] [%s] %s\n", logs[i].Type, logs[i].Description, c.name, logs[i].Time.Format("2006-01-02T15:04:05Z07:00"), logs[i].Log.Error())
	}
	return nil
}
