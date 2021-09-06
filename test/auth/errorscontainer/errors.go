package errorscontainer

import (
	"context"
	"errors"
	"sync"
	"time"
)

type ErrorsContainer struct {
	errors       []Error
	adderrmutex  sync.Mutex
	flushing     chan []Error
	doneflushing bool
	Done         chan struct{}
}

type Error struct {
	Time time.Time
	Err  error
}

type ErrorsFlusher interface {
	Flush([]Error) error
}

func NewErrorsContainer(ctx context.Context, f ErrorsFlusher, capacity int, flushperiod time.Duration, minflushinglen int) (*ErrorsContainer, error) {

	if capacity < 1 {
		return nil, errors.New("capacity < 1")
	}
	if minflushinglen < 1 {
		return nil, errors.New("minflushinglen < 1")
	}
	if minflushinglen > capacity {
		return nil, errors.New("minflushinglen > capacity")
	}

	container := &ErrorsContainer{
		errors:      make([]Error, 0, capacity),
		adderrmutex: sync.Mutex{},
		flushing:    make(chan []Error, 1),
		Done:        make(chan struct{}, 1),
	}

	go errorslistener(ctx, f, container, flushperiod, minflushinglen)

	return container, nil
}
func (errs *ErrorsContainer) AddError(err error) {
	now := time.Now()
	errs.adderrmutex.Lock()
	defer errs.adderrmutex.Unlock()

	if errs.doneflushing || err == nil {
		return
	}
	if len(errs.errors) == cap(errs.errors) {
		errs.flushing <- errs.errors
		errs.errors = make([]Error, 0, cap(errs.errors))
	}
	errs.errors = append(errs.errors, Error{Time: now, Err: err})
}

func errorslistener(ctx context.Context, f ErrorsFlusher, errs *ErrorsContainer, flushperiod time.Duration, minflushinglen int) {

	ticker := time.NewTicker(flushperiod)
	go func() {
		for errpack := range errs.flushing {
			if err := f.Flush(errpack); err != nil {
				//errs.AddError(err)
			}
		}
		errs.Done <- struct{}{}
	}()
	go func() {
		for range ticker.C {
			errs.adderrmutex.Lock()

			if len(errs.errors) >= minflushinglen {
				errs.flushing <- errs.errors
				errs.errors = make([]Error, 0, cap(errs.errors))
			}
			errs.adderrmutex.Unlock()
		}
	}()

	<-ctx.Done()

	errs.adderrmutex.Lock()
	ticker.Stop()

	errs.doneflushing = true
	errs.flushing <- errs.errors
	close(errs.flushing)
	//<-errs.done
	errs.adderrmutex.Unlock()
}
