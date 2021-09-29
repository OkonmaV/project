package main

import (
	"context"
	"sync"

	"io/ioutil"
	"net"
	"project/test/auth/logscontainer"
	"project/test/configurator/gopool"
	"strings"
	"time"

	"github.com/big-larry/suckutils"
	"github.com/mailru/easygo/netpoll"
	"github.com/tarantool/go-tarantool"
)

type Configurator struct {
	services  map[string]*serviceinstance
	hosts     map[string]struct{}
	poller    netpoll.Poller
	pool      *gopool.Pool
	trntlConn *tarantool.Connection
	TrntlErr  chan struct{} //
}

type serviceinstance struct {
	addr   string
	mutex  sync.Mutex
	wsconn net.Conn
}

// TODO: додумать разные инстансы одного сервиса
type trntlTuple struct {
	name     string
	addr     string
	status   bool
	lastseen int64
}

func NewConfigurator(settingspath, tarantooladdr string) (*Configurator, error) {
	trntlConn, err := tarantool.Connect(tarantooladdr, tarantool.Opts{
		// User: ,
		// Pass: ,
		Timeout:       500 * time.Millisecond,
		Reconnect:     1 * time.Second,
		MaxReconnects: 4,
	})
	if err != nil {
		return nil, err
	}
	if err = trntlConn.UpsertAsync("configurator", []interface{}{"conf.configurator", "", true, 0}, []interface{}{
		[]interface{}{"=", "status", true},
		[]interface{}{"=", "lastseen", 0}}).Err(); err != nil {
		return nil, err
	}
	c := &Configurator{trntlConn: trntlConn, TrntlErr: make(chan struct{}), services: make(map[string]*serviceinstance), hosts: map[string]struct{}{}}
	if err := c.readsettings(settingspath); err != nil {
		return nil, err
	}

	c.hosts["127.0.0.1"] = struct{}{}
	c.poller, err = netpoll.New(nil)
	if err != nil {
		return nil, err
	}
	c.pool = gopool.NewPool(5, 1, 1)

	return c, nil
}

func (c *Configurator) CloseTarantoolWithUpdateStatus(l *logscontainer.LogsContainer) error {
	if err := c.trntlConn.UpdateAsync("configurator", "primary", []interface{}{"conf.configurator"}, []interface{}{
		[]interface{}{"=", "status", false},
		[]interface{}{"=", "lastseen", time.Now().Unix()},
	}).Err(); err != nil {
		l.Error("TrntlUpdate", err)
	}
	return c.trntlConn.Close()
}

func (c *Configurator) Serve(ctx context.Context, l *logscontainer.LogsContainer, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	if err := c.trntlConn.UpdateAsync("configurator", "primary", []interface{}{"conf.configurator"}, []interface{}{
		[]interface{}{"=", "addr", addr},
	}).Err(); err != nil {
		return err
	}
	l.Info("Start service", suckutils.ConcatTwo("conf.configurator start listening at ", addr))

	acceptDesc, err := netpoll.HandleListener(ln, netpoll.EventRead|netpoll.EventOneShot)
	if err != nil {
		return err
	}
	accept := make(chan error, 1)

	if err = c.poller.Start(acceptDesc, func(e netpoll.Event) {
		// We do not want to accept incoming connection when goroutine pool is
		// busy. So if there are no free goroutines during 1ms we want to
		// cooldown the server and do not receive connection for some short
		// time.
		err := c.pool.ScheduleTimeout(time.Millisecond, func() {
			conn, err := ln.Accept()
			if err != nil {
				accept <- err
				return
			}

			accept <- nil
			c.handlehttp(conn, l, c.poller, c.pool)
		})
		if err == nil {
			err = <-accept
		}
		if err != nil {
			if err != gopool.ErrScheduleTimeout {
				goto cooldown
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				goto cooldown
			}

			l.Error("Accept", err)

		cooldown:
			delay := 3 * time.Millisecond
			l.Warning("Accept", suckutils.ConcatFour("err: ", err.Error(), "; retrying in ", delay.String()))
			time.Sleep(delay)
		}

		c.poller.Resume(acceptDesc)
	}); err != nil {
		//l.Error("poller.Start", err)
		return err
	}
	<-ctx.Done()
	l.Info("Stop service", "Configurator stopping")
	return c.poller.Stop(acceptDesc)
}

func (c *Configurator) readsettings(settingspath string) error {
	data, err := ioutil.ReadFile(settingspath)
	if err != nil {
		return err
	}
	datastr := string(data)
	lines := strings.Split(datastr, "\n")
	for _, line := range lines {
		if len(line) < 2 || strings.HasPrefix(line, "#") {
			continue
		}
		s := strings.Split(strings.TrimSpace(line), " ")
		if len(s) < 2 {
			continue
		}
		c.services[s[0]] = &serviceinstance{addr: s[1]}
	}
	return nil
}
