package main

import (
	"context"
	"errors"
	"net"
	"os"
	"project/test/connector"
	"project/test/types"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/big-larry/suckutils"
	"github.com/segmentio/fasthash/fnv1a"
)

type service struct {
	name      ServiceName
	outerAddr Address // адрес на котором сервис будет торчать наружу
	status    types.ServiceStatus
	connector connector.Conn
	reconnect bool
	mux       sync.Mutex
	l         types.Logger
}

type services struct {
	list  map[ServiceName]*service_state
	rwmux sync.RWMutex
}

type service_state struct {
	connections []*service
	rwmux       sync.RWMutex
	conf_hash   uint32
}

func (s *services) getServiceState(name ServiceName) *service_state { // я хз как назвать нормально
	s.rwmux.RLock()
	defer s.rwmux.RUnlock()
	return s.list[name]
}

func (state *service_state) initNewConnection(conn net.Conn) error {
	if state == nil {
		return errors.New("unknown service") // да, неочевидна
	}
	state.rwmux.RLock()
	defer state.rwmux.RUnlock()

	for i := 0; i < len(state.connections); i++ {
		state.connections[i].mux.Lock()
		var con connector.Conn
		var err error
		if state.connections[i].status == types.StatusOff {

			if state.connections[i].reconnect {
				if con, err = connector.NewEpollReConnector(conn, state.connections[i]); err != nil { // TODO: где-то InitReconnector сделать
					goto failure
				}
				if err = con.StartServing(); err != nil {
					goto failure
				}
				goto success
			}
		}
	failure:
		state.connections[i].mux.Unlock()
		if err != nil {
			return err
		}
		continue
	success:
		state.connections[i].connector = con
		state.connections[i].status = types.StatusSuspended // status update to suspend
		state.connections[i].mux.Unlock()
		return nil
	}
	return errors.New("no free conns for this service available") // TODO: где-то имя сервиса в логи вписать
}

func (s *services) serveSettings(ctx context.Context, l types.Logger, settingspath string, ticktime time.Duration) {
	filestat, err := os.Stat(settingspath)
	if err != nil {
		panic("[os.stat] error: " + err.Error())
	}
	lastmodified := filestat.ModTime().Unix()
	ticker := time.NewTicker(ticktime)
	for {
		select {
		case <-ctx.Done():
			l.Debug("readsettings", "context done, exiting")
			ticker.Stop()
			return
		case <-ticker.C:
			fs, err := os.Stat(settingspath)
			if err != nil {
				l.Error("os.Stat", err)
			}
			lm := fs.ModTime().Unix()
			if lastmodified < lm {
				if err := s.readSettings(l, settingspath); err != nil {
					l.Error("readsettings", err)
					continue
				}
				lastmodified = lm
			}
		}
	}
}

func (s *services) readSettings(l types.Logger, settingspath string) error {
	data, err := os.ReadFile(settingspath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")

	for n, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}
		ind := strings.Index(line, " ")
		name := ServiceName((line)[:ind])
		current_hash := fnv1a.HashString32((line)[:ind])
		state := s.list[name]
		var addrs []string // шоб лишний раз не нарезать
		if state == nil {
			if addrs = strings.Split((line)[ind+1:], " "); len(addrs) == 0 {
				continue
			}
			state = &service_state{connections: make([]*service, 0, len(addrs))}
		}
		if state.conf_hash == current_hash {
			continue
		}
		if addrs = strings.Split((line)[ind+1:], " "); len(addrs) == 0 {
			continue
		}

		//	loop:
		for i := 0; i < len(addrs); i++ {
			current_address := readAddress(addrs[i])
			if current_address == nil {
				l.Debug("readAddress", suckutils.Concat("incorrect address at line ", strconv.Itoa(n), ": ", addrs[i]))
				continue
			}
			for k := i; k < len(state.connections); k++ { // TODO: СРАВНЕНИЕ КАЖДЫЙ С КАЖДЫМ??????
				//	if state.connections[k].outerAddr.addr==current_address.addr
			}

		}

	}

}
