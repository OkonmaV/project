package suspender

import (
	"sync"
)

type suspend struct {
	onair bool
	rwmux sync.RWMutex

	onSuspend   func(string)
	onUnSuspend func()
}

type Suspendier interface {
	Suspend_checkier
	Suspend(reason string)
	UnSuspend()
	SetFunctions(doAfterSuspend func(reason string), doAfterUnSuspend func())
}

type Suspend_checkier interface {
	OnAir() bool
}

func NewSuspendier(doAfterSuspend func(reason string), doAfterUnSuspend func()) Suspendier {
	return &suspend{onSuspend: doAfterSuspend, onUnSuspend: doAfterUnSuspend}
}

// TODO: rename
func (s *suspend) SetFunctions(doAfterSuspend func(reason string), doAfterUnSuspend func()) {
	s.rwmux.Lock()
	defer s.rwmux.Unlock()
	s.onSuspend = doAfterSuspend
	s.onUnSuspend = doAfterUnSuspend
}

func (s *suspend) OnAir() bool {
	s.rwmux.RLock()
	defer s.rwmux.RUnlock()
	return s.onair
}

func (s *suspend) UnSuspend() {
	s.rwmux.Lock()
	defer s.rwmux.Unlock()
	s.onair = true

	if s.onUnSuspend != nil {
		s.onUnSuspend()
	}
}

func (s *suspend) Suspend(reason string) {
	s.rwmux.Lock()
	defer s.rwmux.Unlock()
	s.onair = false

	if s.onSuspend != nil {
		s.onSuspend(reason)
	}
}
