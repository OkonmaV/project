package main

import (
	"context"
	"errors"

	"io/ioutil"

	"strings"

	"github.com/big-larry/suckutils"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/mailru/easygo/netpoll"
)

type Configurator struct {
	memcConn       *memcache.Client
	poller         netpoll.Poller
	subscriptions  *connections
	localListener  *Listener //unix
	remoteListener *Listener //tcp
}

func NewConfigurator(settingspath, memcaddr string) (*Configurator, error) {
	c := &Configurator{subscriptions: &connections{connectors: make(map[ServiceName][]*Connector)}}
	c.memcConn = memcache.New(memcaddr)
	err := readsettings(c.memcConn, settingspath)
	if err != nil {
		return nil, err
	}
	if c.poller, err = netpoll.New(nil); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Configurator) Serve(ctx context.Context, localunixaddr, remoteipv4addr string) error {
	localln, err := NewListener("unix", localunixaddr, c.handlews)
	if err != nil {
		return err
	}
	remoteln, err := NewListener("tcp", remoteipv4addr, c.handlews)
	if err != nil {
		return err
	}
	if err = c.memcConn.Set(&memcache.Item{Key: ConfServiceName.Local(),
		Value: GenerateMemcStatusValue(ParseIPv4withPort(remoteipv4addr), ParseUnix(localunixaddr), StatusOn)}); err != nil {
		return err
	}
	l.Info("Start service", suckutils.Concat("configurator start listening at ", localunixaddr, " for local, and at ", remoteipv4addr, " for remote"))
	<-ctx.Done()
	if err = c.memcConn.Set(&memcache.Item{Key: ConfServiceName.Local(),
		Value: GenerateMemcStatusValue(ParseIPv4withPort(remoteipv4addr), ParseUnix(localunixaddr), StatusOn)}); err != nil {
		l.Error("Memc Set", err)
	}
	if err = localln.Close(); err != nil {
		l.Error("Listener.Close", err)
	}
	if err = remoteln.Close(); err != nil {
		l.Error("Listener.Close", err)
	}
	l.Info("Stop service", "configurator stopping")
	return nil
}

func readsettings(memcConn *memcache.Client, settingspath string) error {
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
				memcConn.Set(&memcache.Item{Key: ServiceName(s[0]).Local(), Value: []byte{0, 0, 0, 0, 0, 0, byte(StatusOff)}})
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

// func getMyExternalIPv4(l logscontainer.Logger, apiAddrs []string) (IPv4, error) {
// 	for _, uri := range apiAddrs {
// 		resp, err := http.Get(uri)
// 		if err != nil {
// 			l.Warning("ExternalIP API", suckutils.Concat("GET to ", uri, " caused error: ", err.Error()))
// 			resp.Body.Close()
// 			continue
// 		}
// 		ip, err := ioutil.ReadAll(resp.Body)
// 		if err != nil {
// 			l.Warning("ExternalIP API", suckutils.Concat("ReadAll responce from ", uri, " caused error: ", err.Error()))
// 			resp.Body.Close()
// 			continue
// 		}
// 		resp.Body.Close()
// 		if ipv4 := ParseIPv4(string(ip)); ipv4 != nil {
// 			return ipv4, nil
// 		} else {
// 			l.Warning("ExternalIP API", suckutils.Concat("cant parse responce from ", uri, " to an IPv4"))
// 		}
// 	}
// 	return nil, errors.New("non of responces from apis was satisfactory")
// }
