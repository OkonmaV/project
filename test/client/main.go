package main

import (
	"context"
	"project/test/logs"
	"time"
)

const (
	thisservicename string = "testclient"
	confaddr        string = "127.0.0.1:9001"
)

func main() {
	ctx := context.Background()
	logcontainer, _ := logs.NewLoggerContainer(ctx, logs.DebugLevel, 10, time.Second*2)
	consolelogger := &logs.ConsoleLogger{}
	onlinelogger, _ := logs.NewOnlineLogger(logs.DebugLevel)
	go func() {
		for {
			select {
			case l := <-onlinelogger.Flush():
				logcontainer.Write(l.Time, l.Level, l.Name, l.Message)
			case l := <-logcontainer.Flush():
				consolelogger.WriteMany(l)
			}
		}
	}()

}
