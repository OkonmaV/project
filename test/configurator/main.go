package main

import (
	"context"
	"os"
	"os/signal"
	"strings"

	"project/test/epolllistener"
	"project/test/logs"

	"time"

	"github.com/BurntSushi/toml"
)

type config struct {
	ListenLocal    string // куда стучатся сервисы, обслуживаемые этим конфигуратором (могут быть и не локальными)
	ListenExternal string // куда стучатся другие конфигураторы. можно ListenLocal = ListenExternal, если tcp
	Settings       string
}

// TODO: при подрубе удаленных сервисов 100% будут проблемы, поэтому так пока нельзя делать (см структуру Address)
// TODO: "пингпонг" по таймеру между конфигураторами??? / при подключении нового конфигуратора автоматом слать ему подписку / апдейт статуса самого конфигуратора

func main() {
	conf := &config{}
	_, err := toml.DecodeFile("config.toml", conf)
	if err != nil {
		panic("read toml err: " + err.Error())
	}
	if (conf.ListenLocal == "" && conf.ListenExternal == "") || conf.Settings == "" {
		panic("some fields in conf.toml are empty or not specified")
	}

	ctx, cancel := createContextWithInterruptSignal()
	logsctx, logscancel := context.WithCancel(context.Background())

	l, _ := logs.NewLoggerContainer(logsctx, logs.DebugLevel, 10, time.Second*2)
	consolelogger := &logs.ConsoleLogger{}

	go func() {
		for {
			logspack := <-l.Flush()
			consolelogger.WriteMany(logspack)
		}
	}()
	suspender := newSuspendier()
	subs := newSubscriptions(ctx, l, 5, suspender, nil)
	servs := newServices(ctx, l, conf.Settings, suspender, time.Minute*5, subs)
	subs.services = servs

	var local_ln, external_ln listenier
	epolllistener.SetupEpollErrorHandler(func(e error) {
		l.Error("Epoll", e)
		cancel()
	})

	if local_ln, err = newListener((conf.ListenLocal)[:strings.Index(conf.ListenLocal, ":")], (conf.ListenLocal)[strings.Index(conf.ListenLocal, ":")+1:], subs, servs, l); err != nil {
		panic("newListener err: " + err.Error())
	}
	if conf.ListenExternal != "" && conf.ListenExternal != conf.ListenLocal {
		if external_ln, err = newListener((conf.ListenLocal)[:strings.Index(conf.ListenLocal, ":")], (conf.ListenLocal)[strings.Index(conf.ListenLocal, ":")+1:], subs, servs, l); err != nil {
			panic("newListener err: " + err.Error())
		}
	}

	<-ctx.Done()
	l.Debug("Context", "done, exiting")
	local_ln.close()
	if external_ln != nil {
		external_ln.close()
	}
	logscancel()
	time.Sleep(time.Second * 3) // TODO: ждун в логгере
}

func createContextWithInterruptSignal() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, os.Kill)

	go func() {
		<-stop
		cancel()
	}()
	return ctx, cancel
}
