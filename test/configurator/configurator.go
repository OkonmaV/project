package main

import (
	"context"
	"encoding/binary"
	"sync"

	"io/ioutil"
	"net"

	"project/test/configurator/gopool"
	"project/test/logscontainer"
	"strings"
	"time"

	"github.com/big-larry/suckutils"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/mailru/easygo/netpoll"
)

type Configurator struct {
	services map[string]*serviceinstance
	hosts    map[string]struct{}
	poller   netpoll.Poller
	pool     *gopool.Pool
	memcConn *memcache.Client
	//TrntlErr  chan struct{} //
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

func NewConfigurator(settingspath, memcaddr string) (*Configurator, error) {

	c := &Configurator{memcConn: memcache.New(memcaddr), services: make(map[string]*serviceinstance), hosts: map[string]struct{}{}}
	err := c.memcConn.Set(&memcache.Item{Key: "conf.conf", Value: []byte("this")})
	if err != nil {
		return nil, err
	}
	if err = c.readsettings(settingspath); err != nil {
		return nil, err
	}
	if c.poller, err = netpoll.New(nil); err != nil {
		return nil, err
	}
	c.pool = gopool.NewPool(5, 1, 1)
	c.hosts["127.0.0.1"] = struct{}{} // TODO

	return c, nil
}

func (c *Configurator) Serve(ctx context.Context, l *logscontainer.LogsContainer, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	updateStatusToOn(c.memcConn, "conf.conf", "addr")
	l.Info("Start service", suckutils.ConcatTwo("conf.configurator start listening at ", addr))

	acceptDesc, err := netpoll.HandleListener(ln, netpoll.EventRead|netpoll.EventOneShot)
	if err != nil {
		return err
	}
	accept := make(chan error, 1)

	if err = c.poller.Start(acceptDesc, func(e netpoll.Event) {
		err := c.pool.ScheduleTimeout(time.Millisecond, func() {
			conn, err := ln.Accept()
			if err != nil {
				accept <- err
				return
			}
			accept <- nil
			wl := l.Wrap(map[string]string{"remote-addr": conn.RemoteAddr().String()})
			c.handlehttp(conn, wl, c.poller, c.pool)
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
		c.memcConn.Set(&memcache.Item{Key: s[0], Value: []byte(s[1])})
		c.services[s[0]] = &serviceinstance{addr: s[1]}
	}
	return nil
}

func updateStatusToOn(memcConn *memcache.Client, servicename, serviceaddress string) error {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(time.Now().Unix()))
	return memcConn.Set(&memcache.Item{Key: suckutils.ConcatThree(servicename, ".", serviceaddress), Value: b})
}

func updateStatusToOff(memcConn *memcache.Client, servicename, serviceaddress string) error {
	key := suckutils.ConcatThree(servicename, ".", serviceaddress)
	item, err := memcConn.Get(key)
	if err != nil {
		return err
	}
	if len(item.Value) < 8 {
		b := make([]byte, 16)
		b = append(b, item.Value...)
		item.Value = b
	}
	binary.LittleEndian.PutUint64(item.Value[:8], uint64(time.Now().Unix()))
	return memcConn.Set(item)

}
