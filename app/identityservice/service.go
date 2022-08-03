package identityserver

import (
	"confdecoder"
	"context"
	"os"
	"os/signal"
	"project/app/protocol"
	"project/connector"
	"project/logs/encode"
	"project/logs/logger"
	"syscall"
	"time"
)

type Servicier interface {
	CreateHandler(ctx context.Context, l logger.Logger, pubs_getter Publishers_getter) (Handler, error)
}

type Handler interface {
	appReqHandler
	userReqHandler
}

type appReqHandler interface {
	Handle_Token_Request(client_id, secret, code, redirect_uri string) (accessttoken, refreshtoken, userid string, expires int64, errCode protocol.ErrorCode)
	Handle_AppAuth(appname, appid string) (errCode protocol.ErrorCode)
	Handle_AppRegistration(appname string) (appid, secret string, errCode protocol.ErrorCode)
}

type userReqHandler interface {
	Handle_Auth_Request(login, password string, client_id string, redirect_uri string, scope []string) (grant_code string, errCode protocol.ErrorCode)
	Handle_UserRegistration_Request(login, password string) (errCode protocol.ErrorCode)
}

type closer interface {
	Close() error
}

type file_config struct {
	ConfiguratorAddr string
}

type ServiceName string

const pubscheckTicktime time.Duration = time.Second * 5

// TODO: придумать шото для неторчащих наружу сервисов

func InitNewService(servicename ServiceName, config Servicier, handlethreads int, publishers_names ...ServiceName) {
	initNewService(true, servicename, config, handlethreads, publishers_names...)
}
func InitNewServiceWithoutConfigurator(servicename ServiceName, config Servicier, handlethreads int, publishers_names ...ServiceName) {
	if len(publishers_names) > 0 {
		panic("cant use publishers without configurator")
	}
	initNewService(false, servicename, config, handlethreads, publishers_names...)
}

func initNewService(configurator_enabled bool, servicename ServiceName, config Servicier, handlethreads int, publishers_names ...ServiceName) {
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

	connector.SetupEpoll(func(e error) {
		l.Error("epoll OnWaitError", e)
		cancel()
	})
	if err := connector.SetupGopoolHandling(handlethreads, 1, handlethreads); err != nil {
		panic(err)
	}

	var pubs *publishers

	if len(publishers_names) != 0 {
		if pubs, err = newPublishers(ctx, l.NewSubLogger("Publishers"), servStatus, nil, pubscheckTicktime, publishers_names); err != nil {
			panic(err)
		}
	} else {
		servStatus.setPubsStatus(true)
	}

	handler, err := config.CreateHandler(ctx, l.NewSubLogger("Handler"), pubs)
	if err != nil {
		panic(err)
	}

	ln := newListener(ctx, l.NewSubLogger("Listener"), l, handler, servStatus)

	var configurator *configurator

	if configurator_enabled {
		configurator = newConfigurator(ctx, l.NewSubLogger("Configurator"), servStatus, pubs, ln, servconf.ConfiguratorAddr, servicename, time.Second*5)
		if pubs != nil {
			pubs.configurator = configurator
		}
	} else {
		if configurator = newFakeConfigurator(ctx, l.NewSubLogger("FakeConfigurator"), servStatus, ln); configurator == nil {
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
	if closehandler, ok := handler.(closer); ok {
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
