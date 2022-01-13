package main

import "sync"

type suspend struct {
	onair bool
	rwmux sync.RWMutex
}

type suspendier interface {
	suspend_checkier
	suspend()
	unsuspend()
}

type suspend_checkier interface {
	onAir() bool
}

func newSuspendier() suspendier {
	return &suspend{}
}

func (s *suspend) onAir() bool {
	s.rwmux.RLock()
	defer s.rwmux.RUnlock()
	return s.onair
}

func (s *suspend) unsuspend() {
	s.rwmux.Lock()
	defer s.rwmux.Unlock()
	s.onair = true
	return
}

func (s *suspend) suspend() {
	s.rwmux.Lock()
	defer s.rwmux.Unlock()
	s.onair = false
	return
}
