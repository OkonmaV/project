package httpservice

import (
	"confdecoder"
	"context"
	"net"
	"os"
	"os/signal"
	"project/connector"
	"project/logs/encode"
	"project/logs/logger"

	"syscall"
	"time"

	"github.com/big-larry/suckhttp"
)

type ServiceName string

type Servicier interface {
	CreateHandler(ctx context.Context, pubs_getter Publishers_getter) (HTTPService, error)
}

type closer interface {
	Close() error
}

type config_toml struct {
	ConfiguratorAddr string
}

type HTTPService interface {
	// nil response = 500
	Handle(request *suckhttp.Request, logger logger.Logger) (*suckhttp.Response, error)
}

const pubscheckTicktime time.Duration = time.Second * 5

// TODO: ПЕРЕЕХАТЬ КОНФИГУРАТОРА НА НОН-ЕПУЛ КОННЕКТОР
// TODO: придумать шото для неторчащих наружу сервисов

func InitNewService(servicename ServiceName, config Servicier, keepConnAlive bool, handlethreads int, publishers_names ...ServiceName) {
	initNewService(true, servicename, config, keepConnAlive, handlethreads, publishers_names...)
}
func InitNewServiceWithoutConfigurator(servicename ServiceName, config Servicier, keepConnAlive bool, handlethreads int, publishers_names ...ServiceName) {
	if len(publishers_names) > 0 {
		panic("cant use publishers without configurator")
	}
	initNewService(false, servicename, config, keepConnAlive, handlethreads, publishers_names...)
}

func initNewService(configurator_enabled bool, servicename ServiceName, config Servicier, keepConnAlive bool, handlethreads int, publishers_names ...ServiceName) {
	servconf := &config_toml{}
	if err := confdecoder.DecodeFile("config.txt", servconf, config); err != nil {
		panic("reading/decoding config.txt err: " + err.Error())
	}
	if configurator_enabled && servconf.ConfiguratorAddr == "" {
		panic("ConfiguratorAddr in config.toml not specified")
	}

	ctx, cancel := createContextWithInterruptSignal()

	logsflusher := logger.NewFlusher(encode.DebugLevel)
	l := logsflusher.NewLogsContainer(string(servicename))

	servStatus := newServiceStatus()

	if err := connector.SetupEpoll(func(e error) {
		l.Error("epoll OnWaitError", e)
		cancel()
	}); err != nil {
		panic(err)
	}
	// Connector here is only needed for configurator
	// if err := connector.SetupGopoolHandling(handlethreads, 1, handlethreads); err != nil {
	// 	panic(err)
	// }

	var pubs *publishers
	var err error
	if len(publishers_names) != 0 {
		if pubs, err = newPublishers(ctx, l, servStatus, nil, pubscheckTicktime, publishers_names); err != nil {
			panic(err)
		}
	} else {
		servStatus.setPubsStatus(true)
	}
	handler, err := config.CreateHandler(ctx, pubs)
	if err != nil {
		panic(err)
	}

	ln := newListener(ctx, l, servStatus, handlethreads, keepConnAlive, func(conn net.Conn) error {
		request, err := suckhttp.ReadRequest(ctx, conn, time.Minute)
		if err != nil {
			return err
		}
		response, err := handler.Handle(request, l)
		if response == nil {
			response = suckhttp.NewResponse(500, "Internal Server Error")
		}
		if err != nil {
			if writeErr := response.Write(conn, time.Minute); writeErr != nil {
				l.Error("Write response", writeErr)
			}
			return err
		}
		return response.Write(conn, time.Minute)
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

//func handler(ctx context.Context, conn net.Conn) error
