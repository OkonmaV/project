package wsservice

import (
	"confdecoder"
	"context"
	"net"
	"os"
	"os/signal"
	"project/logs/encode"
	"project/logs/logger"
	"project/wsconnector"
	"syscall"
	"time"
)

type ServiceName string

type Servicier interface {
	CreateService(ctx context.Context, pubs_getter Publishers_getter) (WSService, error)
}

type WSService interface {
	CreateNewWsHandler(l logger.Logger) Handler
}

type Handler interface {
	HandleNewConnection(WSconn) error
	wsconnector.WsHandler
}

type WSconn interface {
	wsconnector.Sender
	wsconnector.Informer
	wsconnector.Closer
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
// TODO: конфигурировать кол-во горутин-хендлеров конфигуратором
// TODO: вынести фейковый конфигуратор в отдельную либу

func InitNewService(servicename ServiceName, config Servicier, lnthreads, handlethreads int, publishers_names ...ServiceName) {
	initNewService(true, servicename, config, lnthreads, handlethreads, publishers_names...)
}

// you can specify listenport in config.txt as ListenPort
func InitNewServiceWithoutConfigurator(servicename ServiceName, config Servicier, lnthreads, handlethreads int, publishers_names ...ServiceName) {
	if len(publishers_names) > 0 {
		panic("cant use publishers without configurator")
	}
	initNewService(false, servicename, config, lnthreads, handlethreads, publishers_names...)
}

func initNewService(configurator_enabled bool, servicename ServiceName, config Servicier, lnthreads, handlethreads int, publishers_names ...ServiceName) {
	servconf := &file_config{}
	pfd, err := confdecoder.ParseFile("config.txt")
	if err != nil {
		panic("parsing config.txt err: " + err.Error())
	}
	if err = pfd.DecodeTo(servconf, config); err != nil {
		panic("decoding config.txt err: " + err.Error())
	}

	if configurator_enabled && servconf.ConfiguratorAddr == "" {
		panic("ConfiguratorAddr in config.toml not specified")
	}

	ctx, cancel := createContextWithInterruptSignal()

	logsflusher := logger.NewFlusher(encode.DebugLevel)
	l := logsflusher.NewLogsContainer(string(servicename))

	servStatus := newServiceStatus()

	var pubs *publishers
	//var err error
	if len(publishers_names) != 0 {
		if pubs, err = newPublishers(ctx, l.NewSubLogger("Publishers"), servStatus, nil, pubscheckTicktime, publishers_names); err != nil {
			panic(err)
		}
	} else {
		servStatus.setPubsStatus(true)
	}
	srvc, err := config.CreateService(ctx, pubs)
	if err != nil {
		panic(err)
	}

	if err := wsconnector.SetupEpoll(func(e error) {
		l.Error("epoll OnWaitError", e)
		cancel()
	}); err != nil {
		panic(err)
	}
	if err := wsconnector.SetupGopoolHandling(handlethreads, 1, handlethreads); err != nil {
		panic(err)
	}

	l_lstnr := l.NewSubLogger("Listener")
	ln := newListener(l_lstnr, servStatus, lnthreads, func(conn net.Conn) error {
		wsdata := srvc.CreateNewWsHandler(l.NewSubLogger("Conn"))
		connector, err := wsconnector.NewWSConnectorWithUpgrade(conn, wsdata)
		if err != nil {
			return err
		}

		if err = wsdata.HandleNewConnection(connector); err != nil {
			l_lstnr.Debug("HandleWSCreating", err.Error())
			return err
		}
		if err := connector.StartServing(); err != nil {
			l_lstnr.Debug("StartServing", err.Error())
			connector.ClearFromCache()
			return err
		}
		return nil
	})

	var configurator *configurator

	if configurator_enabled {
		configurator = newConfigurator(ctx, l.NewSubLogger("Configurator"), servStatus, pubs, ln, servconf.ConfiguratorAddr, servicename, time.Second*5)
		if pubs != nil {
			pubs.configurator = configurator
		}
	} else {
		foo := &struct{ ListenPort int }{}
		if err := pfd.DecodeTo(foo); err != nil {
			panic("decoding config.txt err: " + err.Error())
		}
		if configurator = newFakeConfigurator(ctx, foo.ListenPort, l.NewSubLogger("FakeConfigurator"), servStatus, ln); configurator == nil {
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
	logsflusher.Close()
	<-logsflusher.Done()
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
