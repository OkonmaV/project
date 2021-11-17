package main

import (
	"context"
	"io"
	"os"
	"project/test/connector"
	"strconv"
	"sync"
	"time"

	"strings"

	"github.com/big-larry/suckutils"
)

type Configurator struct {
	localservices         *servicesinfo //only for ports info
	localservicesstatuses *servicesinfo
	remoteservices        *servicesinfo
	remoteconfs           *servicesinfo
	subscriptions         *connections
	localListener         *listener //unix
	remoteListener        *listener //tcp
}

type servicesinfo struct {
	//local: порт
	//remote: адрес
	//remoteconf: адрес
	serviceinfo map[ServiceName]map[string]ServiceStatus
	rwmux       sync.RWMutex
}

func NewConfigurator(ctx context.Context, settingspath string, settingsreadticktime time.Duration) (*Configurator, error) {
	c := &Configurator{subscriptions: &connections{connectors: make(map[ServiceName][]*connector.Connector)}}

	if err := c.readsettings(ctx, settingspath); err != nil {
		return nil, err
	}
	go c.servesettings(ctx, settingsreadticktime, settingspath)

	return c, nil
}

func (c *Configurator) Serve(ctx context.Context, localunixaddr, remoteipv4addr string) error {
	localln, err := c.NewListener("unix", localunixaddr, false)
	if err != nil {
		return err
	}
	remoteln, err := c.NewListener("tcp", remoteipv4addr, true)
	if err != nil {
		return err
	}
	l.Info("Start service", suckutils.Concat("configurator start listening at ", localunixaddr, " for local, and at ", remoteipv4addr, " for remote"))
	<-ctx.Done()
	if err = localln.Close(); err != nil {
		l.Error("Listener.Close", err)
	}
	if err = remoteln.Close(); err != nil {
		l.Error("Listener.Close", err)
	}
	l.Info("Stop service", "configurator stopping")
	return nil
}

func (c *Configurator) servesettings(ctx context.Context, ticktime time.Duration, settingspath string) {
	filestat, err := os.Stat(settingspath)
	if err != nil {
		panic("[servesettings][os.stat] error: " + err.Error())
	}
	lastmodified := filestat.ModTime().Unix()
	ticker := time.NewTicker(ticktime)

	for {
		select {
		case <-ctx.Done():
			l.Debug("servesettings", "context done, exiting")
			ticker.Stop()
			return
		case <-ticker.C:
			fs, err := os.Stat(settingspath)
			if err != nil {
				l.Error("os.Stat", err)
			}
			lm := fs.ModTime().Unix()
			if lastmodified < lm {
				if err := c.readsettings(ctx, settingspath); err != nil {
					l.Error("readsettings", err)
					continue
				}
				lastmodified = lm
			}
		}
	}
}

// TODO: при удалении/изменении адреса логики не написано
func (c *Configurator) readsettings(ctx context.Context, settingspath string) error {
	file, err := suckutils.OpenConcurrentFile(ctx, settingspath, time.Second*7)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := readfile(file.File)
	if err != nil {
		return err
	}

	datastr := string(data)
	lines := strings.Split(datastr, "\n")
	for n, line := range lines {
		if /*len(line) < 2 ||*/ strings.HasPrefix(line, "#") {
			continue
		}
		s := strings.Split(strings.TrimSpace(line), " ")
		if len(s) < 2 {
			l.Debug("readsettings", suckutils.ConcatTwo("splitted line length <2, skip line ", strconv.Itoa(n)))
		}
		if s[2] == "*" {
			if _, ok := c.localservices.serviceinfo[ServiceName(s[0])]; !ok {
				c.localservices.rwmux.Lock()
				c.localservices.serviceinfo[ServiceName(s[0])] = make(map[string]ServiceStatus, 2)
				c.localservices.serviceinfo[ServiceName(s[0])]["*"] = 1
				c.localservices.rwmux.Unlock()
			}
			continue
		}
		if ServiceName(s[0]) == ServiceName(ConfServiceName.Remote()) {
			if rconfs, ok := c.remoteconfs.serviceinfo[ServiceName(s[0])]; !ok {
				c.remoteconfs.rwmux.Lock()
				c.remoteconfs.serviceinfo[ServiceName(s[0])] = make(map[string]ServiceStatus, len(s)-1)
				for i := 1; i < len(s); i++ {
					if addr := ParseIPv4withPort(s[i]); addr != nil {
						c.remoteconfs.serviceinfo[ServiceName(s[0])][addr.String()] = StatusOff
					} else {
						l.Debug("readsettings", suckutils.Concat("skip at line ", strconv.Itoa(n), " remoteconf's ", strconv.Itoa(i), " addr: not correctly parsed"))
					}
				}
				c.remoteconfs.rwmux.Unlock()
			} else {
				for i := 1; i < len(s); i++ {
					if addr := ParseIPv4withPort(s[i]); addr != nil {
						if _, ok := rconfs[addr.String()]; !ok {
							c.remoteconfs.rwmux.Lock()
							rconfs[addr.String()] = StatusOff
							c.remoteconfs.rwmux.Unlock()
						}
					} else {
						l.Debug("readsettings", suckutils.Concat("skip at line ", strconv.Itoa(n), " remoteconf's ", strconv.Itoa(i), " addr: not correctly parsed"))
					}
				}

				continue
			}
		}

		if lservs, ok := c.localservices.serviceinfo[ServiceName(s[0])]; !ok {
			c.localservices.rwmux.Lock()
			c.localservices.serviceinfo[ServiceName(s[0])] = make(map[string]ServiceStatus, len(s)-1)
			for i := 1; i < len(s); i++ {
				if addr := ParseIPv4withPort(s[i]); addr != nil {
					c.localservices.serviceinfo[ServiceName(s[0])][addr.Port().String()] = StatusOff
				} else {
					l.Debug("readsettings", suckutils.Concat("skip at line ", strconv.Itoa(n), " service's ", strconv.Itoa(i), " addr: not correctly parsed"))
				}
			}
			c.localservices.rwmux.Unlock()
		} else {
			for i := 1; i < len(s); i++ {
				if addr := ParseIPv4withPort(s[i]); addr != nil {
					if _, ok := lservs[addr.String()]; !ok {
						c.localservices.rwmux.Lock()
						lservs[addr.String()] = StatusOff
						c.localservices.rwmux.Unlock()
					}
				} else {
					l.Debug("readsettings", suckutils.Concat("skip at line ", strconv.Itoa(n), " service's ", strconv.Itoa(i), " addr: not correctly parsed"))
				}
			}
			continue
		}
	}
	return nil
}

func readfile(f *os.File) ([]byte, error) { // = os.ReadFile
	var size int
	if info, err := f.Stat(); err == nil {
		size64 := info.Size()
		if int64(int(size64)) == size64 {
			size = int(size64)
		}
	}
	size++
	if size < 512 {
		size = 512
	}

	data := make([]byte, 0, size)
	for {
		if len(data) >= cap(data) {
			d := append(data[:cap(data)], 0)
			data = d[:len(data)]
		}
		n, err := f.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return data, err
		}
	}
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
