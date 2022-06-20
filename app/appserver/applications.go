package main

import (
	"context"
	"errors"
	"project/app/protocol"
	"project/connector"
	"project/logs/logger"
	"project/types/configuratortypes"
	"strconv"

	"time"

	"github.com/big-larry/suckutils"
)

var errNoAliveConns = errors.New("no alive conns")

type applications struct {
	list []app // zero index is always nil
	//sync.RWMutex
	appupdates chan appupdate

	configurator *configurator
	l            logger.Logger
}

type appupdate struct {
	name   ServiceName
	netw   string
	addr   string
	status configuratortypes.ServiceStatus
}

// call before configurator created
func newApplications(ctx context.Context, l logger.Logger, configurator *configurator, pubscheckTicktime time.Duration, size int) (*applications, func()) {
	a := &applications{configurator: configurator, l: l, list: make([]app, size+1), appupdates: make(chan appupdate, 1)}
	for _, info := range appsinfo {
		if _, err := a.newApp(info.id, clients, info.name); err != nil {
			return nil, nil, err
		}
	}
	updateWorkerStart := make(chan struct{}, 0)

	go a.appsUpdateWorker(ctx, l.NewSubLogger("AppsUpdateWorker"), updateWorkerStart, pubscheckTicktime)
	return a, func() { close(updateWorkerStart) }

}

func (apps *applications) newApp(appid protocol.AppID, clients *clientsConnsList, appname ServiceName) (*app, error) {
	// apps.Lock()
	// defer apps.Unlock()

	if appid == 0 {
		return nil, errors.New("zero appID")
	}
	if int(appid) >= len(apps.list) {
		return nil, errors.New("weird appID (appID is bigger than num of apps)")
	}

	if apps.list[appid].conns != nil {
		apps.list[appid] = app{
			servicename: appname,
			appid:       appid,
			conns:       make([]*connector.EpollReConnector, 0, 1),
			clients:     clients,
			l:           apps.l.NewSubLogger("App", suckutils.ConcatTwo("AppID:", strconv.Itoa(int(appid))), string(appname))}
		return &apps.list[appid], nil
	} else {
		return nil, errors.New("app is already created")
	}
}

func (apps *applications) update(appname ServiceName, netw, addr string, status configuratortypes.ServiceStatus) {
	apps.appupdates <- appupdate{name: appname, netw: netw, addr: addr, status: status}
}

func (apps *applications) appsUpdateWorker(ctx context.Context, l logger.Logger, updateWorkerStart <-chan struct{}, appsscheckTicktime time.Duration) {
	<-updateWorkerStart
	l.Debug("UpdateLoop", "started")
	ticker := time.NewTicker(pubscheckTicktime)
loop:
	for {
		select {
		case <-ctx.Done():
			l.Debug("Context", "context done, exiting")
			return
		case update := <-apps.appupdates:
			//apps.RLock()
			// чешем список
			for i := 1; i < len(apps.list); i++ {
				if apps.list[i].servicename == update.name {
					// если есть в списке
					//apps.RUnlock()

					// чешем список подключений
					for i := 0; i < len(apps.list[i].conns); i++ {
						// если нашли в списке подключений
						if apps.list[i].conns[i].RemoteAddr().String() == update.addr {
							// если нужно отрубать
							if update.status == configuratortypes.StatusOff {
								apps.list[i].Lock()
								apps.list[i].conns[i].CancelReconnect()
								apps.list[i].conns[i].Close(errors.New("update from configurator"))
								//apps.list[i].conns = append(apps.list[i].conns[:i], apps.list[i].conns[i+1:]...)
								apps.list[i].conns = apps.list[i].conns[:i+copy(apps.list[i].conns[i:], apps.list[i].conns[i+1:])]
								l.Debug("Update", suckutils.ConcatFour("due to update, closed conn to \"", string(apps.list[i].servicename), "\" from ", update.addr))

								apps.list[i].Unlock()

								l.Debug("Update", suckutils.Concat("app \"", string(apps.list[i].servicename), "\" from ", update.addr, " updated to", update.status.String()))
								continue loop

							} else if update.status == configuratortypes.StatusOn { // если нужно подрубать = ошибка
								l.Error("Update", errors.New(suckutils.Concat("appupdate to status_on for already updated status_on for \"", string(apps.list[i].servicename), "\" from ", update.addr)))
								continue loop

							} else { // если кривой апдейт = ошибка
								l.Error("Update", errors.New(suckutils.Concat("unknown statuscode: ", strconv.Itoa(int(update.status)), " at update app \"", string(apps.list[i].servicename), "\" from ", update.addr)))
								continue loop
							}
						}
					}

					// если не нашли в списке подключений:

					// если нужно подрубать
					if update.status == configuratortypes.StatusOn {
						apps.list[i].connect(update.netw, update.addr)
						continue loop

					} else if update.status == configuratortypes.StatusOff { // если нужно отрубать = ошибка
						l.Error("Update", errors.New(suckutils.Concat("appupdate to status_off for already updated status_off for \"", string(apps.list[i].servicename), "\" from ", update.addr)))
						continue loop

					} else { // если кривой апдейт = ошибка
						l.Error("Update", errors.New(suckutils.Concat("unknown statuscode: ", strconv.Itoa(int(update.status)), "at update pub \"", string(apps.list[i].servicename), "\" from ", update.addr)))
						continue loop
					}
				}
			}
			//apps.RUnlock()

			// если нет в списке = ошибка и отписка

			l.Error("Update", errors.New(suckutils.Concat("appupdate for non-subscription \"", string(update.name), "\", sending unsubscription")))

			appname_byte := []byte((update.name))
			message := append(append(make([]byte, 0, 2+len(appname_byte)), byte(configuratortypes.OperationCodeUnsubscribeFromServices), byte(len(appname_byte))), appname_byte...)
			if err := apps.configurator.send(message); err != nil {
				l.Error("configurator.Send", err)
			}

		case <-ticker.C:
			empty_appsnames := make([]ServiceName, 0, len(apps.list))
			empty_appnames_totallen := 0
			//apps.RLock()
			for _, app := range apps.list {
				app.RLock()
				if len(app.conns) == 0 {
					empty_appsnames = append(empty_appsnames, app.servicename)
					empty_appnames_totallen += len(app.servicename)
				}
				app.RUnlock()
			}
			//apps.RUnlock()
			message := make([]byte, 1, empty_appnames_totallen+len(empty_appsnames)+1)
			message[0] = byte(configuratortypes.OperationCodeSubscribeToServices)
			for _, pub_name := range empty_appsnames {
				pub_name_byte := []byte(pub_name)
				message = append(append(message, byte(len(pub_name_byte))), pub_name_byte...)
			}
			if err := apps.configurator.send(message); err != nil {
				l.Error("configurator.Send", err)
			}
		}
	}
}

// func (apps *applications) GetAllAppsIDs() []appID {
// 	apps.RLock()
// 	defer apps.RUnlock()
// 	res := make([]appID, 0, len(apps.list))
// 	for appid := range apps.list {
// 		res = append(res, appid)
// 	}
// 	return res
// }

func (apps *applications) GetAllAppNames() []ServiceName {
	// apps.RLock()
	// defer apps.RUnlock()
	res := make([]ServiceName, 0, len(apps.list)-1)
	for _, app := range apps.list {
		res = append(res, app.servicename)
	}
	return res
}

func (apps *applications) Get(appid protocol.AppID) (*app, error) {
	if appid == 0 || int(appid) >= len(apps.list) {
		return nil, errors.New(suckutils.ConcatThree("impossible appid (must be 0<connuid<=len(apps.list)): \"", strconv.Itoa(int(appid)), "\""))
	}
	return &apps.list[appid], nil
}
