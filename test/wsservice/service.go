package wsservice

import (
	"confdecoder"
	"context"
	"net"
	"os"
	"os/signal"
	"project/test/connector"
	"project/test/logs"
	"project/test/types"
	"project/test/wsconnector"
	"syscall"
	"time"
)

type ServiceName string

type Servicier interface {
	CreateHandlers(ctx context.Context, pubs_getter Publishers_getter) (Service, error)
}

type Service interface {
	CreateNewWsData(l types.Logger) Handler
}

type Handler interface {
	HandleWSCreating(wsconnector.Sender) error
	wsconnector.WsHandler
}

type closer interface {
	Close() error
}

type file_config struct {
	ConfiguratorAddr string
}

const pubscheckTicktime time.Duration = time.Second * 5

// TODO: исправить жопу с логами
// TODO: придумать шото для неторчащих наружу сервисов

func InitNewService(servicename ServiceName, config Servicier, keepConnAlive bool, threads int, publishers_names ...ServiceName) {
	initNewService(true, servicename, config, keepConnAlive, threads, publishers_names...)
}
func InitNewServiceWithoutConfigurator(servicename ServiceName, config Servicier, keepConnAlive bool, threads int, publishers_names ...ServiceName) {
	if len(publishers_names) > 0 {
		panic("cant use publishers without configurator")
	}
	initNewService(false, servicename, config, keepConnAlive, threads, publishers_names...)
}

func initNewService(configurator_enabled bool, servicename ServiceName, config Servicier, keepConnAlive bool, threads int, publishers_names ...ServiceName) {
	servconf := &file_config{}
	if err := confdecoder.DecodeFile("config.txt", servconf, config); err != nil {
		panic("reading/decoding config.txt err: " + err.Error())
	}
	if configurator_enabled && servconf.ConfiguratorAddr == "" {
		panic("ConfiguratorAddr in config.toml not specified")
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

	servStatus := newServiceStatus()

	connector.SetupEpoll(func(e error) {
		l.Error("epoll OnWaitError", e)
		cancel()
	})

	var pubs *publishers
	var err error
	if len(publishers_names) != 0 {
		if pubs, err = newPublishers(ctx, l, servStatus, nil, pubscheckTicktime, publishers_names); err != nil {
			panic(err)
		}
	} else {
		servStatus.setPubsStatus(true)
	}
	srvc, err := config.CreateHandlers(ctx, pubs)
	if err != nil {
		panic(err)
	}

	if err := wsconnector.SetupEpoll(nil); err != nil {
		panic(err)
	}
	if err := wsconnector.SetupGopoolHandling(threads, 1, threads); err != nil {
		panic(err)
	}

	ln := newListener(l, servStatus, threads, keepConnAlive, func(conn net.Conn) error {
		println("HERE")
		wsdata := srvc.CreateNewWsData(l)
		connector, err := wsconnector.NewWSConnector(conn, wsdata)
		if err != nil {
			return err
		}

		if err = wsdata.HandleWSCreating(connector); err != nil {
			l.Debug("HandleWSCreating", err.Error())
			return err
		}
		if err := connector.StartServing(); err != nil {
			l.Debug("StartServing", err.Error())
			connector.ClearFromCache()
			return err
		}
		return nil
	})

	var configurator *configurator

	if configurator_enabled {
		configurator = newConfigurator(ctx, l, servStatus, pubs, ln, servconf.ConfiguratorAddr, servicename, time.Second*5)
		if pubs != nil {
			pubs.configurator = configurator
		}
	} else {
		if configurator = newFakeConfigurator(ctx, l, servStatus, ln); configurator == nil {
			cancel()
		}
	}

	//ln.configurator = configurator
	servStatus.setOnSuspendFunc(configurator.onSuspend)
	servStatus.setOnUnSuspendFunc(configurator.onUnSuspend)

	select {
	case <-ctx.Done():
		l.Info("Shutdown", "reason: context done")
		break
	case <-configurator.terminationByConfigurator:
		l.Info("Shutdown", "reason: termination by configurator")
		break
	}

	ln.close()

	if closehandler, ok := srvc.(closer); ok {
		if err = closehandler.Close(); err != nil {
			l.Error("CloseFunc", err)
		}
	}
	logscancel()
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

//func handler(ctx context.Context, conn net.Conn) error
