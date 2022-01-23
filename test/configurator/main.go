package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"project/test/connector"
	"project/test/epolllistener"
	"project/test/logs"
	"project/test/suspender"
	"project/test/types"

	"time"

	"github.com/BurntSushi/toml"
	"github.com/big-larry/suckutils"
)

type config struct {
	ListenLocal    string // куда стучатся сервисы, обслуживаемые этим конфигуратором (могут быть и не локальными)
	ListenExternal string // куда стучатся другие конфигураторы. можно ListenLocal = ListenExternal, если tcp
	Settings       string
}

const connectors_gopool_size int = 5
const settingsCheckTicktime time.Duration = time.Minute * 5
const reconnectsCheckTicktime time.Duration = time.Second * 10
const reconnectsTargetBufSize int = 4

// TODO: при подрубе удаленных сервисов 100% будут проблемы, поэтому так пока нельзя делать (см структуру Address)
// TODO: "пингпонг" по таймеру между конфигураторами??? / при подключении нового конфигуратора автоматом слать ему подписку / апдейт статуса самого конфигуратора
// TODO: реконнектор здесь есть, но работать будет через жопу (т.е. не будет работать)
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

	subs := newSubscriptions(ctx, l, 5, nil)
	suspender := suspender.NewSuspendier(
		func(reason string) {
			l.Warning("ownStatus", suckutils.ConcatTwo("suspended, reason: ", reason))
			subs.updateItself(types.StatusSuspended)
		}, func() {
			l.Warning("ownStatus", "unsuspended")
			subs.updateItself(types.StatusOn)
		})

	servs := newServices(ctx, l, conf.Settings, suspender, settingsCheckTicktime, subs)
	subs.services = servs

	connector.SetupEpoll(func(e error) {
		l.Error("Epoll", e)
		cancel()
	})
	connector.SetupGopoolHandling(connectors_gopool_size, 1, connectors_gopool_size/2)
	connector.InitReconnection(ctx, reconnectsCheckTicktime, reconnectsTargetBufSize, reconnectsTargetBufSize/2)

	epolllistener.SetupEpollErrorHandler(func(e error) {
		l.Error("Epoll", e)
		cancel()
	})

	var local_ln, external_ln listenier
	if local_ln, err = newListener((conf.ListenLocal)[:strings.Index(conf.ListenLocal, ":")], (conf.ListenLocal)[strings.Index(conf.ListenLocal, ":")+1:], subs, servs, l); err != nil {
		panic("newListener err: " + err.Error())
	}
	if conf.ListenExternal != "" && conf.ListenExternal != conf.ListenLocal {
		if external_ln, err = newListener((conf.ListenLocal)[:strings.Index(conf.ListenLocal, ":")], (conf.ListenLocal)[strings.Index(conf.ListenLocal, ":")+1:], subs, servs, l); err != nil {
			panic("newListener err: " + err.Error())
		}
	}
	suspender.UnSuspend() // TODO: КТО АНСУСПЕНД ДЕЛАТЬ БУДЕТ??

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
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		cancel()
	}()
	return ctx, cancel
}
