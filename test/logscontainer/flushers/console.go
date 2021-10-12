package flushers

import (
	"fmt"
	"os"
	"project/test/logscontainer"
)

type Console struct {
	name string
}

func NewConsoleFlusher(name string) *Console {
	return &Console{name: name}
}

func (c *Console) Flush(logs []logscontainer.Log) error {
	for _, log := range logs {
		if len(log.Tags) == 0 {
			fmt.Fprintf(os.Stdout, "[%s] [%s] [%s] [%s] %s\n", log.Lvl.String(), log.Description, c.name, log.Time.Format("2006-01-02T15:04:05Z07:00"), log.Log.Error())
		} else {
			fmt.Fprintf(os.Stdout, "[%s] [%s] [%s] [%s] [%s] %s\n", log.Lvl.String(), log.Description, c.name, log.Time.Format("2006-01-02T15:04:05Z07:00"), log.Tags.String(), log.Log.Error())
		}
	}
	return nil
}
