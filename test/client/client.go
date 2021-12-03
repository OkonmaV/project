package client

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"project/test/connector"
	"strings"
	"sync"

	"github.com/big-larry/suckutils"
)

type Service struct {
	name      ServiceName
	pubs      map[ServiceName][]*pubConnectorinfo // TODO: рассмотреть возможность переделать в массив
	pubsrwmux *sync.RWMutex                       // будет же работать?
	pubupdate chan []byte
	//instancesFeedback []chan []byte
	handler    ClientHandleSub
	OnAir      bool
	OnAirMux   sync.RWMutex
	Sendtoconf func([]byte) error
	listener   *listener
}

type pubConnectorinfo struct {
	address     string
	network     string
	servicename ServiceName
	l           logger
	s           *Service
	//handle      func([]byte) error
	send    func([]byte) error
	close   func(error)
	closech chan struct{} // чтобы остановить реконнект
}

type Connectorinfo struct {
	servicename   ServiceName
	l             logger
	getremoteaddr ClientGetConAddr
	handle        ClientHandleSub
	send          func([]byte) error
}

type logger interface {
	Debug(string, string)
	Info(string, string)
	Warning(string, string)
	Error(string, error)
}

type ClientPubResponce chan []byte
type ClientDoOnPubDisconnect func()
type ClientHandleSub func(logger, ServiceName, ClientGetConAddr, ClientSendResponce, []byte) ([]byte, error) // TODO: возврат ошибки вызовет закрытие коннекшна, обсудить
type ClientGetConAddr func() (string, string)
type ClientSendResponce func([]byte) error

func (ci *pubConnectorinfo) handleconf(payload []byte) error {
	return nil
}

func (ci *Connectorinfo) handlesub(payload []byte) error {
	if len(payload) < 2 {
		return connector.ErrWeirdData
	}
	responce, err := ci.handle(ci.l, ci.servicename, ci.getremoteaddr, nil, payload[2:])
	if err != nil {
		return err // TODO: вызовет закрытие коннекшна, обсудить
	}
	respbuf := make([]byte, 0, 2+len(responce))
	if err = ci.send(append(append(respbuf, payload[:2]...), responce...)); err != nil {
		ci.l.Error("Send", err)
	}
	return err
}

func (ci *Connectorinfo) handlesubclose(err error) {
	_, addr := ci.getremoteaddr()
	ci.l.Debug("Connector", suckutils.Concat("con with ", string(ci.servicename), " from ", addr, " closed, reason: ", err.Error()))
	return
}

//type ClientSendToSub func([]byte, []byte) error

//TODO: сделать каналы для фидбэка или аналог
func NewService(l logger, servicename ServiceName, confaddress string, instancesnum int, subshandler ClientHandleSub, pubnames ...ServiceName) (*Service, error) {
	if instancesnum < 1 {
		return nil, errors.New("instancesnum must be greater than 0")
	}
	if len(servicename) == 0 {
		return nil, errors.New("empty servicename")
	}
	connector.SetupGopoolHandling(instancesnum, 1, instancesnum)

	service := &Service{}
	service.pubs = make(map[ServiceName][]*pubConnectorinfo)
	for _, name := range pubnames {
		service.pubs[name] = make([]*pubConnectorinfo, 0, 3)
	}

	confconinfo := &pubConnectorinfo{s: service, network: (confaddress)[:strings.Index(confaddress, ":")], address: (confaddress)[strings.Index(confaddress, ":")+1:], servicename: ConfServiceName, l: l, closech: make(chan struct{})}
	confconn, err := net.Dial(confconinfo.network, confconinfo.address)
	if err != nil {
		return nil, err
	}

	if confcon, err := connector.NewConnector(confconn, confconinfo.handlepub, confconinfo.handlepubclose); err != nil {
		return nil, err
	} else {
		confconinfo.send = confcon.Send
		if err = confcon.StartServing(); err != nil {
			return nil, err
		}
	}
	if err = confconinfo.send([]byte(servicename)); err != nil {
		return nil, err
	}
	// TODO: req pubs from conf here?

	return service, nil
}

// type pubsettingsupdate struct {
// 	pubname   ServiceName
// 	network   string
// 	address   string
// 	newstatus bool
// }

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
