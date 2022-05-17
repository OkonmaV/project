package main

import (
	"context"
	"errors"
	"net/http"
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
	localservices map[ServiceName]*serviceinstance
	//localsubs           map[ServiceName]ServiceName // можно и прям ссылку на структуру пихать
	remoteconfigurators map[string]*serviceinstance // KEY=ADDRESS without port // МОЖЕТ В СЛАЙС ИХ ПИХАТЬ?
	poller              netpoll.Poller
	pool                *gopool.Pool
	memcConn            *memcache.Client
	externalIPapis      []string
	myexternalIP        IPv4
}

type serviceinstance struct {
	addr   IPv4withPort
	mutex  sync.Mutex
	wsconn net.Conn
}

func NewConfigurator(settingspath, memcaddr string) (*Configurator, error) {

	c := &Configurator{
		memcConn:            memcache.New(memcaddr),
		localservices:       make(map[ServiceName]*serviceinstance),
		remoteconfigurators: make(map[string]*serviceinstance)}
	err := c.memcConn.Set(&memcache.Item{Key: "local.conf", Value: []byte("this")})
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

	return c, nil
}

func (c *Configurator) Serve(ctx context.Context, l *logscontainer.LogsContainer, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	byteaddr := ParseIPv4withPort(addr)
	if byteaddr == nil {
		return errors.New("can listen addr, but cant parseipv4withport it")
	}
	// TODO: add memc sub
	if err = c.memcConn.Set(&memcache.Item{Key: "local.conf", Value: byteaddr.WithStatus(StatusOn)}); err != nil {
		return err
	}
	l.Info("Start service", suckutils.ConcatTwo("configurator start listening at ", addr))

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
			wl := l.Wrap(map[logscontainer.Tag]string{logscontainer.TagRemoteAddr: conn.RemoteAddr().String()})
			c.handleHTTP(conn, wl, c.poller, c.pool)
		})
		if err == nil {
			err = <-accept
		}
		// TODO: test timeout
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
	l.Info("Stop service", "configurator stopping")
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
			if len(s) == 1 {
				c.memcConn.Set(&memcache.Item{Key: ServiceName(s[0]).Local(), Value: []byte{0, 0, 0, 0, 0, 0, byte(StatusOff)}})
				c.localservices[ServiceName(s[0])] = &serviceinstance{}
			}
			continue
		}
		if ServiceName(s[0]) == ServiceName(ConfServiceName.Remote()) {
			if len(s) < 2 {
				continue
			}
			var remoteconfs []byte
			if len(s) > 2 {
				remoteconfs = make([]byte, 0, ((len(s) - 1) * 6))
			} else {
				remoteconfs = make([]byte, 0, 6)
			}

			for i := 1; i < len(s); i++ { // skip s[0]
				addr := ParseIPv4withPort(s[i])
				if addr == nil {
					return errors.New(suckutils.ConcatTwo("wrong address format of ", string(ConfServiceName.Remote())))
				}
				if Addr(addr).IsLocalhost() {
					return errors.New(suckutils.ConcatTwo(string(ConfServiceName.Remote()), " must not be at localhost"))
				}
				remoteconfs = append(remoteconfs, addr...)
				c.remoteconfigurators[addr[:4].String()] = &serviceinstance{addr: addr} // key=addr without port
			}

			c.memcConn.Set(&memcache.Item{Key: s[0], Value: remoteconfs}) // TODO: может конфигураторы они в мемкэше нахер и не нужны?
			continue
		}
		addr := ParseIPv4withPort(s[1])
		if addr == nil {
			return errors.New(suckutils.ConcatTwo("wrong address format of service ", s[0]))
		}
		if !Addr(addr).IsLocalhost() {
			return errors.New(suckutils.ConcatTwo("not localhost at service ", s[0]))
		}
		c.memcConn.Set(&memcache.Item{Key: ServiceName(s[0]).Local(), Value: addr.WithStatus(StatusOff)})
		c.localservices[ServiceName(s[0])] = &serviceinstance{addr: addr}
	}
	return nil
}

func getMyExternalIPv4(l logscontainer.Logger, apiAddrs []string) (IPv4, error) {
	for _, uri := range apiAddrs {
		resp, err := http.Get(uri)
		if err != nil {
			l.Warning("ExternalIP API", suckutils.Concat("GET to ", uri, " caused error: ", err.Error()))
			resp.Body.Close()
			continue
		}
		ip, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			l.Warning("ExternalIP API", suckutils.Concat("ReadAll responce from ", uri, " caused error: ", err.Error()))
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		if ipv4 := ParseIPv4(string(ip)); ipv4 != nil {
			return ipv4, nil
		} else {
			l.Warning("ExternalIP API", suckutils.Concat("cant parse responce from ", uri, " to an IPv4"))
		}
	}
	return nil, errors.New("non of responces from apis was satisfactory")
}
