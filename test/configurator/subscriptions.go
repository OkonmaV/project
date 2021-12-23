package main

import (
	"errors"
	"sync"
)

type subscriptions struct {
	services map[ServiceName][]*service
	rwmux    sync.RWMutex
}

const maxFreeSpace int = 3 // reslice subscriptions.services, when cap-len > maxFreeSpace

// no err when already subscribed
func (subs *subscriptions) add(pubName ServiceName, sub *service) error {
	if sub == nil {
		return errors.New("nil sub")
	}

	if len(pubName) == 0 {
		return errors.New("empty pubname")
	}

	subs.rwmux.Lock()
	defer subs.rwmux.Unlock()

	if _, ok := subs.services[pubName]; ok {
		for _, subbed := range subs.services[pubName] {
			if subbed == sub {
				return nil // already subscribed
			}
		}
		if cap(subs.services[pubName]) == len(subs.services[pubName]) { // reslice
			subs.services[pubName] = append(make([]*service, 0, len(subs.services[pubName])+maxFreeSpace), subs.services[pubName]...)
		}

		subs.services[pubName] = append(subs.services[pubName], sub)
	} else {
		subs.services[pubName] = append(make([]*service, 1), sub)
	}
	return nil
}

// no err when not subscribed
func (subs *subscriptions) remove(pubName ServiceName, sub *service) error {
	if sub == nil {
		return errors.New("nil sub")
	}

	if len(pubName) == 0 {
		return errors.New("empty pubname")
	}

	subs.rwmux.Lock()
	defer subs.rwmux.Unlock()

	if _, ok := subs.services[pubName]; ok {
		for i := 0; i < len(subs.services[pubName]); i++ {
			if subs.services[pubName][i] == sub {
				subs.services[pubName] = append(subs.services[pubName][:i], subs.services[pubName][i+1:]...)
				break
			}
		}
	}
	if cap(subs.services[pubName])-len(subs.services[pubName]) > maxFreeSpace { // reslice after overflow
		subs.services[pubName] = append(make([]*service, 0, len(subs.services[pubName])+maxFreeSpace), subs.services[pubName]...)
	}
	return nil
}

func (subs *subscriptions) get(pubName ServiceName) []*service {
	if len(pubName) == 0 {
		return nil
	}

	subs.rwmux.RLock()
	defer subs.rwmux.RUnlock()

	if _, ok := subs.services[pubName]; ok {
		return append(make([]*service, len(subs.services[pubName])), subs.services[pubName]...)
	}
	return nil
}
