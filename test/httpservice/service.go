package httpservice

import (
	"context"
	"net"
	"os"
	"os/signal"
	"project/test/connector"
	"project/test/logs"
	"project/test/types"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/big-larry/suckhttp"
)

type ServiceName string

type Servicier interface {
	CreateHandler(ctx context.Context, pubs_getter Publishers_getter) (HTTPService, error)
}

type handlecloser interface {
	Close()
}

type config_toml struct {
	ConfiguratorAddr string
}

type HTTPService interface {
	// nil responce = 500
	Handle(request *suckhttp.Request, logger types.Logger) (*suckhttp.Response, error)
}

const pubscheckTicktime time.Duration = time.Second * 5

// TODO: исправить жопу с логами
// TODO: придумать шото для неторчащих наружу сервисов
// TODO: написать самому замену томлу сраному
func InitNewService(servicename ServiceName, config Servicier, keepConnAlive bool, threads int, publishers_names ...ServiceName) {
	conf := &config_toml{}
	if _, err := toml.DecodeFile("config.toml", conf); err != nil {
		panic("read toml err: " + err.Error())
	}
	if conf.ConfiguratorAddr == "" {
		panic("ConfiguratorAddr in config.toml not specified")
	}
	if _, err := toml.DecodeFile("config.toml", config); err != nil { // эта ебала отдает мапу, но она в привате
		panic("read toml err: " + err.Error())
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
	handler, err := config.CreateHandler(ctx, pubs)
	if err != nil {
		panic(err)
	}

	ln := newListener(ctx, l, servStatus, nil, threads, keepConnAlive, func(conn net.Conn) error {
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
	configurator := newConfigurator(ctx, l, servStatus, pubs, ln, conf.ConfiguratorAddr, servicename, time.Second*5)
	if pubs != nil {
		pubs.configurator = configurator
	}
	ln.configurator = configurator
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

	if handle_closer, ok := handler.(handlecloser); ok {
		handle_closer.Close()
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
