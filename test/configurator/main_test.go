package main_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"project/test/auth/logscontainer"
	"project/test/auth/logscontainer/flushers"
	"strings"
	"sync"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/gobwas/ws"
)

type config struct {
	Listen    string
	Settings  string
	Memcached string
}

func readTomlConfig(path string, conf *config) error {
	if _, err := toml.DecodeFile(path, conf); err != nil {
		return err
	}
	//fmt.Println("config: ", conf)
	return nil
}

func main() {
	conf := &config{}
	if err := readTomlConfig("config.toml", conf); err != nil {
		fmt.Println("reading toml err:", err)
		return
	}

	ctx, cancel := httpservice.CreateContextWithInterruptSignal()

	l, err := logscontainer.NewLogsContainer(ctx, flushers.NewConsoleFlusher("CNFG"), 1, time.Second, 1)
	if err != nil {
		fmt.Println("logs init err:", err)
		return
	}
	defer func() {
		cancel()
		<-l.Done
	}()

}

func (configurator *Configurator) Serve(ctx context.Context, network, address string, connectionAlive bool, maxconnections int) error {
	l, err := net.Listen(network, address)
	if err != nil {
		return err
	}
	listenerLocker := sync.Mutex{}
	done := make(chan error, 1)

	goroutines := make(chan struct{}, maxconnections) // Ограничитель горутин
	group := sync.WaitGroup{}                         // Все запросы будут выполенены
	connections := make([]net.Conn, maxconnections)
	conmux := sync.Mutex{}

	go func() {
		<-ctx.Done()
		listenerLocker.Lock()
		err := l.Close()
		l = nil
		listenerLocker.Unlock()
		conmux.Lock()
		for i, c := range connections {
			if c != nil {
				connections[i].Close()
			}
		}
		conmux.Unlock()
		group.Wait() // Ждем завершения обработки всех запросов
		done <- err
	}()

	for {
		listenerLocker.Lock()
		if l == nil {
			break
		}
		listenerLocker.Unlock()

		fd, err := l.Accept()
		if err != nil {
			fmt.Println("accept:", err)
			continue
		}
		group.Add(1)
		goroutines <- struct{}{} // Ограничивает количество горутин
		conmux.Lock()
		ncon := -1
		for {
			for i, c := range connections {
				if c == nil {
					connections[i] = fd
					ncon = i
					break
				}
			}
			if ncon == -1 {
				time.Sleep(time.Millisecond)
				continue
			}
			break
		}
		conmux.Unlock()

		go func(conn net.Conn, nconn int) {
			fmt.Println(suckutils.ConcatTwo("Service ", address), suckutils.Concat("Open connection from ", conn.LocalAddr().String(), " to ", conn.RemoteAddr().String()))

			for {
				if err := configurator.handler(ctx, conn); err != nil {
					fmt.Println("Handle "+conn.RemoteAddr().String(), err)
					break
				}
				if !connectionAlive {
					break
				}
				conn.SetDeadline(time.Time{})
			}
			fmt.Println(suckutils.ConcatTwo("Service ", address), suckutils.Concat("Connection closing from ", conn.LocalAddr().String(), " to ", conn.RemoteAddr().String()))

			if err := conn.Close(); err != nil {
				logger.Error("Close", err)
			}
			group.Done()
			<-goroutines
			conmux.Lock()
			conn.Close()
			connections[nconn] = nil
			conmux.Unlock()
			// logger.Debug("Service "+address, "end for")
		}(fd, ncon)
	}

	return <-done
}

func (configurator *Configurator) handler(ctx context.Context, conn net.Conn) error {
	request, err := suckhttp.ReadRequest(ctx, conn, time.Minute)
	if err != nil {
		return err
	}

	if strings.Contains(request.GetHeader("Upgrade"), "websocket") {
		if request.GetMethod() != suckhttp.GET {
			return errors.New(suckutils.ConcatTwo("wrong method in ws call to", request.Uri.Path))
		}
		servname := strings.Trim(request.Uri.Path, "/")
		if _, ok := configurator.services[servname]; !ok {
			return errors.New(suckutils.ConcatTwo("unknown service name in ws req", request.Uri.Path))
		}

		if _, err = ws.Upgrade(conn); err != nil {
			//conn.Close()
			return err
		}
		if s, ok := configurator.websockets[servname]; ok { //??
			configurator.websockets[servname] = append(s, conn)
		} else {
			configurator.websockets[servname] = []net.Conn{conn}
		}

	}

	fmt.Println(suckutils.ConcatFour("Readed from ", request.GetRemoteAddr(), " for ", request.Time.String()))

	response, err := handler.Handle(request, l)
	if err != nil {
		l.Error(suckutils.ConcatTwo(logsName, ": handle"), err)
		if response == nil {
			response = suckhttp.NewResponse(500, "Internal Server Error")
		}
		if writeErr := response.Write(conn, time.Minute); writeErr != nil {
			l.Error(suckutils.ConcatTwo(logsName, ": write response"), writeErr)
		}
		return err
	}
	//logger.Debug("Service", "Writing response...")
	err = response.Write(conn, time.Minute)
	if err != nil {
		l.Error(suckutils.ConcatTwo(logsName, ": write response"), err)
	} else {
		l.Debug(logsName, "Done")
	}
	l.Close()
	return err
}
