package httpservice

import (
	"context"
	"errors"
	"net"
	"project/test/connector"
	"project/test/suspender"
	"project/test/types"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/big-larry/suckutils"
)

// TODO: рассмотреть идею о white и grey листах паблишеров

type address struct {
	netw types.NetProtocol
	addr string
}

type publishers struct {
	list       map[ServiceName]*Publisher
	mux        sync.Mutex
	pubupdates chan pubupdate

	configurator configuratorier
	l            types.Logger
}

type Publisher struct {
	conn        net.Conn
	servicename ServiceName
	addresses   []address
	rwmux       sync.RWMutex
}

type configuratorier interface {
	// if you want 100%-sending of your message - do it yourself
	Send([]byte) error
}

type pubupdate struct {
	name   ServiceName
	addr   address
	status types.ServiceStatus
}

func (pubs *publishers) publishersWorker(ctx context.Context, ownStatus suspender.Suspendier, checkTicktime time.Duration) {
	ticker := time.NewTicker(checkTicktime)

loop:
	for {
		select {
		case <-ctx.Done():
			pubs.l.Debug("publishersWorker", "context done, exiting")
			return
		case update := <-pubs.pubupdates:
			// если есть в мапе
			pubs.mux.Lock()
			if pub, ok := pubs.list[update.name]; ok {
				pubs.mux.Unlock()
				// чешем список адресов
				for i := 0; i < len(pub.addresses); i++ {
					// если нашли в списке адресов
					if update.addr.netw == pub.addresses[i].netw && update.addr.addr == pub.addresses[i].addr {
						// если нужно удалять из списка адресов
						if update.status == types.StatusOff || update.status == types.StatusSuspended {
							pub.rwmux.Lock()
							pub.addresses = append(pub.addresses[:i], pub.addresses[i+1:]...)
							pub.rwmux.Unlock()
							pubs.l.Debug("publishersWorker", suckutils.Concat("pub \"", string(update.name), "\" from ", update.addr.addr, " updated to", update.status.String()))
							continue loop

						} else if update.status == types.StatusOn { // если нужно добавлять в список адресов = ошибка
							pubs.l.Error("publishersWorker", errors.New(suckutils.Concat("recieved pubupdate to status_on for already updated status_on for \"", string(update.name), "\" from ", update.addr.addr)))
							continue loop

						} else { // если кривой апдейт = ошибка
							pubs.l.Error("publishersWorker", errors.New(suckutils.Concat("unknown statuscode: ", strconv.Itoa(int(update.status)), "at update pub \"", string(update.name), "\" from ", update.addr.addr)))
							continue loop
						}
					}
				}
				// если не нашли в списке адресов

				// если нужно добавлять в список адресов
				if update.status == types.StatusOn {
					pub.rwmux.Lock()
					pub.addresses = append(pub.addresses, update.addr)
					pub.rwmux.Unlock()
					continue loop

				} else if update.status == types.StatusOff || update.status == types.StatusSuspended { // если нужно удалять из списка адресов = ошибка
					pubs.l.Error("publishersWorker", errors.New(suckutils.Concat("recieved pubupdate to status_suspend/off for already updated status_suspend/off for \"", string(update.name), "\" from ", update.addr.addr)))
					continue loop

				} else { // если кривой апдейт = ошибка
					pubs.l.Error("publishersWorker", errors.New(suckutils.Concat("unknown statuscode: ", strconv.Itoa(int(update.status)), "at update pub \"", string(update.name), "\" from ", update.addr.addr)))
					continue loop
				}

			} else { // если нет в мапе = ошибка и отписка
				pubs.mux.Unlock()
				pubs.l.Error("publishersWorker", errors.New(suckutils.Concat("recieved update for non-publisher \"", string(update.name), "\", sending unsubscription")))

				pubname_byte := []byte(update.name)
				message := append(append(make([]byte, 0, 2+len(update.name)), byte(types.OperationCodeUnsubscribeFromServices), byte(len(pubname_byte))), pubname_byte...)
				if err := pubs.configurator.Send(connector.FormatBasicMessage(message)); err != nil {
					pubs.l.Error("publishersWorker", errors.New(suckutils.ConcatTwo("sending unsubscription to configurator error: ", err.Error())))
				}
			}

		case <-ticker.C:
			empty_pubs := make([]string, 0, 1)
			empty_pubs_len := 0
			//pubs.rwmux.RLock()
			pubs.mux.Lock()
			for pub_name, pub := range pubs.list {
				if len(pub.addresses) == 0 {
					empty_pubs = append(empty_pubs, string(pub_name))
					empty_pubs_len += len(pub_name)
				}
			}
			pubs.mux.Unlock()
			//pubs.rwmux.RUnlock()
			if len(empty_pubs) != 0 {
				ownStatus.Suspend(suckutils.ConcatTwo("no publishers with names: ", strings.Join(empty_pubs, ", ")))
				message := make([]byte, 1, 1+empty_pubs_len+len(empty_pubs))
				message[0] = byte(types.OperationCodeSubscribeToServices)
				for _, pubname := range empty_pubs {
					message = append(message, byte(len(pubname)))
					message = append(message, []byte(pubname)...)
				}
				if err := pubs.configurator.Send(connector.FormatBasicMessage(message)); err != nil {
					pubs.l.Error("Publishers", errors.New(suckutils.ConcatTwo("sending subscription to configurator error: ", err.Error())))
				}
			}
		}
	}
}

func (pubs *publishers) NewPublisher(name ServiceName) (*Publisher, error) {
	pubs.mux.Lock()
	defer pubs.mux.Unlock()

	if _, ok := pubs.list[name]; !ok {
		p := &Publisher{servicename: name, addresses: make([]address, 0, 1)}
		pubs.list[name] = p
		return p, nil
	} else {
		return nil, errors.New("publisher already inited")
	}
}
