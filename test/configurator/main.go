package main

import (
	"context"
	"os"
	"os/signal"

	"project/test/logs"

	"time"

	"github.com/BurntSushi/toml"
)

type config struct {
	ListenUNIX string
	ListenTCP  string
	Settings   string
	Memcached  string
}

var l *logs.LoggerContainer

func main() {
	conf := &config{}
	if _, err := toml.DecodeFile("config.toml", conf); err != nil {
		println("read toml err:", err)
		return
	}
	if (conf.ListenUNIX == "" && conf.ListenTCP == "") || conf.Settings == "" {
		println("some fields in conf.toml are empty or not specified")
	}

	ctx, cancel := createContextWithInterruptSignal()
	logsctx, logscancel := context.WithCancel(context.Background())

	l, _ = logs.NewLoggerContainer(logsctx, logs.DebugLevel, 10, time.Second*2)
	consolelogger := &logs.ConsoleLogger{}
	//l, err := logscontainer.NewLogsContainer(logsctx, flushers.NewConsoleFlusher("CNFG"), 1, time.Second, 1)
	go func() {
		for {
			logspack := <-l.Flush()
			consolelogger.WriteMany(logspack)
		}
	}()
	defer func() {
		cancel()
		logscancel()
		time.Sleep(time.Second * 3) // TODO: ждун в логгере
	}()

	c, err := NewConfigurator(conf.Settings, conf.Memcached)
	if err != nil {
		l.Error("NewConfigurator", err)
		return
	}
	defer func() {

	}()
	if err = c.Serve(ctx, conf.ListenUNIX, conf.ListenTCP); err != nil {
		l.Error("Serve", err)
	}

}

func createContextWithInterruptSignal() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		<-stop
		cancel()
	}()
	return ctx, cancel
}
