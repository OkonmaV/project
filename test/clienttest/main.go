package main

import (
	"context"
	"encoding/binary"
	"net"
	"os"
	"os/signal"
	"project/test/connector"
	"project/test/logs"
	"sync"
	"time"

	"github.com/big-larry/suckutils"
)

const (
	thisservicename string = "testclient"
	confaddr        string = "/tmp/conftest.sock"
	instancesnum    int    = 3
)

type service struct {
	pubs              map[ServiceName][]*pubcon
	pubsrwmux         sync.RWMutex
	instancesFeedback []chan []byte
}

type pubcon struct {
	address string
	network string
	cancel  chan struct{}
	coninfo *Connectorinfo
}

type Connectorinfo struct {
	Servicename   ServiceName
	l             logger
	getremoteaddr func() (string, string)
	handle        ClientHandleSub
	send          func([]byte) error
}

type logger interface {
	Debug(string, string)
	Info(string, string)
	Warning(string, string)
	Error(string, error)
}

type ClientDoOnPubDisconnect func()
type ClientHandleSub func([]byte, []byte) error
type ClientSendToSub func([]byte, []byte) error

// TODO: сделать так, чтобы subwaiterprefix(нумерация запросов) прокидывался в send'ы под капотом (динамически изменять send в Connectorinfo нельзя,
// ибо в коннекторе теперь кошерный мьютекс только на Close = читаем из коннекшна многопоточно)
func (ci *Connectorinfo) handlesub(payload []byte) error {
	if len(payload) < 2 {
		return connector.ErrWeirdData
	}
	return ci.handle(payload[:2], payload[2:])
}

func (ci *Connectorinfo) handlesubclose(err error) {
	_, addr := ci.getremoteaddr()
	ci.l.Debug("Connector", suckutils.Concat("con with ", string(ci.Servicename), " from ", addr, " closed, reason: ", err.Error()))
	return
}

func (ci *Connectorinfo) Send(subwaiterprefix, payload []byte) error {
	newpayload := make([]byte, 2+len(payload))
	return ci.send(append(append(newpayload, subwaiterprefix...), payload...))
}

func main() {
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
	defer func() {
		cancel()
		logscancel()
		time.Sleep(time.Second * 3) // TODO: ждун в логгере
	}()

	// TODO: conn to conf here

}

type listener struct {
	listener net.Listener
	handler  ClientHandleSub
	l        logger
}

func (s *service) Listen(l logger, network, address string, handler ClientHandleSub) error {
	if network == "unix" {
		if err := os.RemoveAll(address); err != nil {
			return err
		}
	}
	ln, err := net.Listen(network, address)
	if err != nil {
		return err
	}
	listener := &listener{listener: ln, handler: handler}
	go listener.accept()
	return nil
}

func (listener *listener) accept() {
	for {
		conn, err := listener.listener.Accept()
		if err != nil {
			listener.l.Error("accept", err)
			return
		}
		conn.SetReadDeadline(time.Now().Add(time.Second * 2))
		buf := make([]byte, 4)
		_, err = conn.Read(buf)
		if err != nil {
			conn.Close()
			continue
		}

		buf = make([]byte, binary.BigEndian.Uint32(buf))
		_, err = conn.Read(buf)
		if err != nil {
			conn.Close()
			continue
		}
		name := ServiceName(buf)
		coninfo := &Connectorinfo{Servicename: name}

		if con, err := connector.NewConnector(conn, coninfo.handlesub, coninfo.handlesubclose); err != nil {
			conn.Close()
			listener.l.Error("NewConnector", err)
			continue
		} else {
			coninfo.handle = listener.handler
			coninfo.getremoteaddr = con.GetRemoteAddr
			coninfo.send = con.Send
			if err = con.StartServing(); err != nil {
				listener.l.Error("StartServing", err)
			}

			listener.l.Info("Connected", suckutils.ConcatThree(string(name), " from ", conn.RemoteAddr().String()))
		}
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
