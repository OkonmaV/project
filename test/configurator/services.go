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

type services struct {
	list  map[ServiceName]*service_state
	rwmux sync.RWMutex
}

func newServices() *services {
	return &services{list: make(map[ServiceName]*service_state)}
}

func (s *services) getServiceState(name ServiceName) *service_state { // я хз как назвать нормально
	s.rwmux.RLock()
	defer s.rwmux.RUnlock()
	return s.list[name]
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

func (s *services) readSettings(l types.Logger, settingspath string) error { // TODO: test this
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

		conf_name := ServiceName(strings.ToLower((line)[:ind])) // перегоняем имя в строчные
		conf_hash := fnv1a.HashString32((line)[ind+1:])         // хэшим строку после имени сервиса
		var rawconf_addrs []string

		state := s.list[conf_name]
		if state == nil { // система не знает про такой сервис
			if rawconf_addrs = strings.Split((line)[ind+1:], " "); len(rawconf_addrs) == 0 {
				continue
			}
			state = &service_state{connections: make([]*service, 0, len(rawconf_addrs))}
			s.rwmux.Lock()
			s.list[conf_name] = state
			s.rwmux.Unlock()
		}
		if state.conf_hash == conf_hash { // если хэш в порядке, пропускаем эту строку
			continue
		}
		state.conf_hash = conf_hash

		if rawconf_addrs = strings.Split((line)[ind+1:], " "); len(rawconf_addrs) == 0 {
			continue
		}

		// TODO: оптимизировать эту всю херь ниже, если возможно

		var conf_enableReconnect bool
		conf_addrs := make([]*Address, 0, len(rawconf_addrs))
		for _, a := range rawconf_addrs { // читаем адреса из settings.conf = проверяем их на ошибки
			if a == ReconnectEnabledFlag { // строка включающая реконнект идет сразу после названия // обсуждаемо
				conf_enableReconnect = true
			}
			if addr := readAddress(a); addr != nil {
				conf_addrs = append(conf_addrs, addr)
			} else {
				l.Warning("readAddress", suckutils.ConcatFour("incorrect address at line ", strconv.Itoa(n), ": ", a))
			}
		}

		for i := 0; i < len(state.connections); i++ { // к нам приехал ревизор outer-портов и реконнекта
			var addrVerified, reconnectVerified bool

			if state.connections[i].reconnect == conf_enableReconnect {
				reconnectVerified = true
			}
			for k := 0; k < len(conf_addrs); k++ {
				if state.connections[i].outerAddr.equal(conf_addrs[k]) {
					addrVerified = true
					conf_addrs = append(conf_addrs[:k], conf_addrs[k+1:]...) // удаляем сошедшийся адрес
					break
				}
			}
			if !addrVerified { // адрес не нашелся в settings
				state.rwmux.Lock()
				state.connections[i].connector.Close(errors.New("settings.conf configuration changed"))
				state.connections = append(state.connections[:i], state.connections[i+1:]...)
				state.rwmux.Unlock()
				i--
			}
			if !reconnectVerified { // настройка реконнекта не сошлась
				state.connections[i].reconnect = conf_enableReconnect
				state.connections[i].connector.Close(errors.New("settings.conf configuration changed")) // сервис к нам сам переподрубится, и мы подрубим его с новым коннектором
			}
		}

		for i := 0; i < len(conf_addrs); i++ { // если остались адреса, еще не присутствующие в системе, мы их добавляем
			state.rwmux.Lock()
			state.connections = append(state.connections, newService(conf_name, conf_addrs[i], conf_enableReconnect, l))
			state.rwmux.Unlock()
		}

	}

	return nil

}

type service_state struct { // TODO: куды подпищеков впихнем?
	connections []*service
	rwmux       sync.RWMutex
	conf_hash   uint32
}

func (state *service_state) initNewConnection(conn net.Conn) error {
	if state == nil {
		return errors.New("unknown service") // да, неочевидна
	}
	state.rwmux.RLock()
	defer state.rwmux.RUnlock()

	for i := 0; i < len(state.connections); i++ {
		state.connections[i].statusmux.Lock()
		var con connector.Conn
		var err error
		if state.connections[i].status == types.StatusOff {

			if state.connections[i].reconnect {
				if con, err = connector.NewEpollReConnector(conn, state.connections[i]); err != nil { // TODO: где-то InitReconnector сделать
					goto failure
				}
				goto success
			}
			if con, err = connector.NewEpollConnector(conn, state.connections[i]); err != nil {
				goto failure
			}
			goto success
		}
	failure:
		state.connections[i].statusmux.Unlock()
		if err != nil {
			return err
		}
		continue
	success:
		if err = con.StartServing(); err != nil {
			goto failure
		}
		state.connections[i].connector = con
		state.connections[i].status = types.StatusSuspended // status update to suspend
		state.connections[i].statusmux.Unlock()
		return nil
	}
	return errors.New("no free conns for this service available") // TODO: где-то имя сервиса в логи вписать
}

type service struct {
	name      ServiceName
	outerAddr *Address // адрес на котором сервис будет торчать наружу
	status    types.ServiceStatus
	statusmux sync.RWMutex
	connector connector.Conn
	reconnect bool
	l         types.Logger

	subs *subscriptions
}

func newService(name ServiceName, outerAddr *Address, reconnect bool, l types.Logger) *service {
	return &service{name: name, outerAddr: outerAddr, l: l}
}

func (s *service) changeStatus(newStatus types.ServiceStatus, subs []*service) {
	s.statusmux.Lock()
	defer s.statusmux.Unlock()

	if s.status == newStatus {
		s.l.Warning("changeStatus", "trying to change already changed status")
		return
	}

	if s.status == types.StatusOn || newStatus == types.StatusOn { // уведомлять о смене сорта нерабочести (eg суспенд) не нужно
		addr_byte := types.FormatAddress(s.outerAddr.netw, s.outerAddr.addr)
		message := make([]byte, 0, len(addr_byte)+2)
		message[0] = byte(types.OperationCodeUpdatePubStatus)
		message[1] = byte(newStatus)
		message = append(message, addr_byte...)

		for i := 0; i < len(subs); i++ {
			if !subs[i].connector.IsClosed() {
				if err := subs[i].connector.Send(message); err != nil {
					subs[i].connector.Close(err)
				}
			}
		}
	}
	s.status = newStatus
}
