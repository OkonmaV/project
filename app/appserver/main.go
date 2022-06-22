package main

import (
	"confdecoder"
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"project/app/protocol"
	"project/connector"
	"project/logs/encode"
	"project/logs/logger"
	"project/wsconnector"
	"syscall"
	"time"

	"github.com/big-larry/suckutils"
)

type ServiceName string

type file_config struct {
	ConfiguratorAddr string
}

const pubscheckTicktime time.Duration = time.Second * 5
const reconnectTimeout time.Duration = time.Second * 3
const clientsConnectionsLimit = 100 // max = 16777215
const clientsServingThreads = 5
const appsServingThreads = 5
const listenerAcceptThreads = 2

// TODO: конфигурировать кол-во горутин-хендлеров конфигуратором

const thisservicename ServiceName = "applicationservice"

func main() {
	servconf := &file_config{}
	pfd, err := confdecoder.ParseFile("config.txt")
	if err != nil {
		panic("parsing config.txt err: " + err.Error())
	}
	if err := pfd.DecodeTo(servconf, servconf); err != nil {
		panic("decoding config.txt err: " + err.Error())
	}

	if servconf.ConfiguratorAddr == "" {
		panic("ConfiguratorAddr in config.toml not specified")
	}
	pfdapps, err := confdecoder.ParseFile("apps.index")
	if err != nil {
		panic("parsing apps.index err: " + err.Error())
	}
	if len(pfdapps.Keys) == 0 {
		panic("no apps to work with (empty apps.index)")
	}

	ctx, cancel := createContextWithInterruptSignal()

	logsflusher := logger.NewFlusher(encode.DebugLevel)
	l := logsflusher.NewLogsContainer(string(thisservicename))

	connector.InitReconnection(ctx, reconnectTimeout, 1, 1)
	wsconnector.SetupEpoll(func(e error) {
		l.Error("epoll OnWaitError", e)
		cancel()
	})
	connector.SetupEpoll(func(e error) {
		l.Error("epoll OnWaitError", e)
		cancel()
	})
	if err := wsconnector.SetupGopoolHandling(clientsServingThreads, 1, clientsServingThreads); err != nil {
		panic(err)
	}
	if err := connector.SetupGopoolHandling(appsServingThreads, 1, appsServingThreads); err != nil {
		panic(err)
	}

	servStatus := newServiceStatus()

	appslist := make([]struct {
		AppID   protocol.AppID `json:"appid"`
		AppName string         `json:"appname"`
	}, len(pfdapps.Keys))

	apps, startAppsUpdateWorker := newApplications(ctx, l.NewSubLogger("Apps"), nil, nil, pubscheckTicktime, len(pfdapps.Keys))

	clients := newClientsConnsList(clientsConnectionsLimit, apps)
	for i, appname := range pfdapps.Keys {
		appsettings, err := os.ReadFile(suckutils.ConcatTwo(appname, ".settings"))
		if err != nil {
			panic(err) // TODO:???????
		}
		appslist = append(appslist, struct {
			AppID   protocol.AppID `json:"appid"`
			AppName string         `json:"appname"`
		}{AppID: protocol.AppID(i + 1), AppName: appname})

		if _, err := apps.newApp(protocol.AppID(i+1), appsettings, clients, ServiceName(appname)); err != nil {
			panic(err)
		}
	}
	appsIndex, err := json.Marshal(appslist)
	if err != nil {
		panic(err)
	}
	apps.appsIndex = appsIndex

	ln := newListener(l.NewSubLogger("Listener"), l.NewSubLogger("Client"), servStatus, apps, clients, listenerAcceptThreads)

	configurator := newConfigurator(ctx, l.NewSubLogger("Configurator"), startAppsUpdateWorker, servStatus, apps, ln, servconf.ConfiguratorAddr, thisservicename)
	apps.configurator = configurator
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
