package main

import (
	"context"
	"errors"
	"project/test/connector"
	"project/test/types"
	"strconv"
	"sync"

	"github.com/big-larry/suckutils"
)

type subscriptions struct {
	subs_list   map[ServiceName][]*service
	rwmux       sync.RWMutex
	services    servicesier
	pubsUpdates chan pubStatusUpdate

	l types.Logger
	//ownStatus   subscriptionsier
}

type subscriptionsier interface {
	updatePub([]byte, []byte, types.ServiceStatus, bool) error
	subscribe(*service, ...ServiceName) error
	unsubscribe(ServiceName, *service) error
	getSubscribers(ServiceName) []*service
}

func newSubscriptions(ctx context.Context, l types.Logger, pubsUpdatesQueue int /*, ownStatus suspender.Suspendier*/, services servicesier, ownSubscriptions ...ServiceName) *subscriptions {
	if pubsUpdatesQueue == 0 {
		panic("pubsUpdatesQueue must be > 0")
	}
	subs := &subscriptions{subs_list: make(map[ServiceName][]*service), services: services, pubsUpdates: make(chan pubStatusUpdate, pubsUpdatesQueue), l: l}
	go subs.pubsUpdatesSendingWorker(ctx)
	return subs
}

// TODO:
// func (subs *subscriptions) ownSubsChecker(ctx context.Context, ownStatus suspendier, checkTicktime time.Duration) {
// 	ticker := time.NewTicker(checkTicktime)
// 	for {
// 		select {
// 		case <-ctx.Done():
// 			subs.l.Debug("ownSubsChecker", "context done, exiting")
// 			return
// 		case <-ticker.C:

// 		}
// 	}
// }

type pubStatusUpdate struct {
	servicename []byte
	address     []byte
	status      types.ServiceStatus
	sendToConfs bool
}

func (subs *subscriptions) pubsUpdatesSendingWorker(ctx context.Context) {
	if subs.pubsUpdates == nil {
		panic("subs.pubUpdates chan is nil")
	}
	for {
		select {
		case <-ctx.Done():
			subs.l.Debug("pubStatusUpdater", suckutils.ConcatTwo("context done, exiting. unhandled updates: ", strconv.Itoa(len(subs.pubsUpdates))))
			// можно вычистить один раз канал апдейтов для разблокировки хэндлеров, но зачем
			return
		case update := <-subs.pubsUpdates:
			if len(update.servicename) == 0 || len(update.address) == 0 {
				subs.l.Error("pubStatusUpdater", errors.New("unformatted update, skipped"))
				continue
			}
			if subscriptors := subs.getSubscribers(ServiceName(update.servicename)); len(subscriptors) != 0 {
				payload := types.ConcatPayload(types.FormatOpcodeUpdatePubMessage(update.servicename, update.address, update.status))
				message := append(append(make([]byte, 1, len(payload)+1), byte(types.OperationCodeUpdatePubs)), payload...)
				if update.sendToConfs {
					sendToMany(connector.FormatBasicMessage(message), subscriptors)
				} else {
					for _, subscriptor := range subscriptors {
						if subscriptor.connector.IsClosed() || subscriptor.name == ServiceName(types.ConfServiceName) {
							continue
						}
						if err := subscriptor.connector.Send(message); err != nil {
							subscriptor.connector.Close(err)
						}
					}
				}
			}
		}
	}
}

func (subs *subscriptions) updatePub(servicename []byte, address []byte, newstatus types.ServiceStatus, sendUpdateToConfs bool) error {
	if len(servicename) == 0 || len(address) == 0 {
		return errors.New("empty/nil servicename/address")
	}
	subs.pubsUpdates <- pubStatusUpdate{servicename: servicename, address: address, status: newstatus, sendToConfs: sendUpdateToConfs}
	return nil
}

const maxFreeSpace int = 3 // reslice subscriptions.services, when cap-len > maxFreeSpace

// no err when already subscribed
func (subs *subscriptions) subscribe(sub *service, pubnames ...ServiceName) error {
	if sub == nil {
		return errors.New("nil sub")
	}

	if len(pubnames) == 0 {
		return errors.New("empty pubname")
	}

	subs.rwmux.Lock()
	defer subs.rwmux.Unlock()

	formatted_updateinfos := make([][]byte, 0, len(pubnames)+2)

	for _, pubname := range pubnames {
		if sub.name == pubname && sub.name != ServiceName(types.ConfServiceName) { // avoiding self-subscription
			return errors.New("service cant subscribe to itself")
		}
		if _, ok := subs.subs_list[pubname]; ok {
			for _, alreadysubbed := range subs.subs_list[pubname] {
				if alreadysubbed == sub { // already subscribed
					continue
				}
			}
			if cap(subs.subs_list[pubname]) == len(subs.subs_list[pubname]) { // reslice, also allocation here if not yet
				subs.subs_list[pubname] = append(make([]*service, 0, len(subs.subs_list[pubname])+maxFreeSpace+1), subs.subs_list[pubname]...)
			}
			subs.subs_list[pubname] = append(subs.subs_list[pubname], sub)

			sub.l.Debug("subs", suckutils.ConcatThree("subscribed to \"", string(pubname), "\""))
		} else {
			subs.subs_list[pubname] = append(make([]*service, 1), sub)
		}

		pubname_byte := []byte(pubname)
		if confs_state := subs.services.getServiceState(ServiceName(types.ConfServiceName)); confs_state != nil { // sending subscription to other confs
			confs := confs_state.getAllServices()
			if len(confs) != 0 {
				message := connector.FormatBasicMessage(append(append(make([]byte, 0, len(pubname_byte)+2), byte(types.OperationCodeSubscribeToServices), byte(len(pubname_byte))), pubname_byte...))
				sendToMany(message, confs)
			}
		}

		if state := subs.services.getServiceState(pubname); state != nil { // getting alive local pubs
			addrs := state.getAllOutsideAddrsWithStatus(types.StatusOn)
			if len(addrs) != 0 {

				for _, addr := range addrs {
					formatted_updateinfos = append(formatted_updateinfos, types.FormatOpcodeUpdatePubMessage(pubname_byte, types.FormatAddress(addr.netw, addr.addr), types.StatusOn))
				}

			}
			sub.l.Debug("subs", suckutils.ConcatThree("no alive local services with name \"", string(pubname), "\""))
		}
	}
	if len(formatted_updateinfos) != 0 {
		updateinfos := types.ConcatPayload(formatted_updateinfos...)
		message := append(make([]byte, 1, len(updateinfos)+1), updateinfos...)
		message[0] = byte(types.OperationCodeUpdatePubs)
		return sub.connector.Send(connector.FormatBasicMessage(message))
	}
	return nil
}

// no err when not subscribed
func (subs *subscriptions) unsubscribe(pubName ServiceName, sub *service) error {
	if sub == nil {
		return errors.New("nil sub")
	}

	if len(pubName) == 0 {
		return errors.New("empty pubname")
	}

	subs.rwmux.Lock()
	defer subs.rwmux.Unlock()

	if _, ok := subs.subs_list[pubName]; ok {
		for i := 0; i < len(subs.subs_list[pubName]); i++ {
			if subs.subs_list[pubName][i] == sub {
				subs.subs_list[pubName] = append(subs.subs_list[pubName][:i], subs.subs_list[pubName][i+1:]...)
				break
			}
		}
	}
	if cap(subs.subs_list[pubName])-len(subs.subs_list[pubName]) > maxFreeSpace { // reslice after overflow
		subs.subs_list[pubName] = append(make([]*service, 0, len(subs.subs_list[pubName])+maxFreeSpace), subs.subs_list[pubName]...)
	}
	return nil
}

func (subs *subscriptions) getSubscribers(pubName ServiceName) []*service {
	if len(pubName) == 0 {
		return nil
	}

	subs.rwmux.RLock()
	defer subs.rwmux.RUnlock()

	if _, ok := subs.subs_list[pubName]; ok {
		return append(make([]*service, len(subs.subs_list[pubName])), subs.subs_list[pubName]...)
	}
	return nil
}
