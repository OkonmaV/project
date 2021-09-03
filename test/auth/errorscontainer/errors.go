package errorscontainer

import (
	"context"
	"errors"
	"sync"
	"time"
)

type ErrorsContainer struct {
	errors      []Error
	adderrmutex sync.Mutex
	flushing    chan []Error
	AddCount    int
	FlushCount  int
}

type Error struct {
	Time    time.Time
	AddTime time.Time
	Err     error
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
	}

	go errorslistener(ctx, f, container, flushperiod, minflushinglen)

	return container, nil
}
func (errs *ErrorsContainer) AddError(err error) {
	now := time.Now()
	errs.adderrmutex.Lock()
	defer errs.adderrmutex.Unlock()
	if err == nil {
		return
	}
	if len(errs.errors) == cap(errs.errors) { //?
		errs.flushing <- errs.errors
		errs.FlushCount += len(errs.errors)
		errs.errors = make([]Error, 0, cap(errs.errors))
	}
	errs.errors = append(errs.errors, Error{Time: now, AddTime: time.Now(), Err: err})
	errs.AddCount++
}

func errorslistener(ctx context.Context, f ErrorsFlusher, errs *ErrorsContainer, flushperiod time.Duration, minflushinglen int) {

	ticker := time.NewTicker(flushperiod)
	go func() {
		for {
			select {
			case errpack := <-errs.flushing:
				if err := f.Flush(errpack); err != nil {
					//errs.AddError(err)
				}
			case <-ctx.Done():
				errs.adderrmutex.Lock()
				errs.FlushCount += len(errs.errors)
				f.Flush(errs.errors)
				ticker.Stop()
				errs.adderrmutex.Unlock()
				return

			}
		}
	}()

	for range ticker.C {
		errs.adderrmutex.Lock()

		if len(errs.errors) >= minflushinglen {
			errs.flushing <- errs.errors
			errs.FlushCount += len(errs.errors)

			errs.errors = make([]Error, 0, cap(errs.errors))
		}
		errs.adderrmutex.Unlock()
	}

	// for {
	// 	select {
	// 	case <-ctx.Done():
	// 		ticker.Stop()
	// 		return

	// 	case <-ticker.C:
	// 		errs.adderrmutex.Lock()

	// 		if len(errs.errors) >= minflushinglen {
	// 			errs.flushing <- errs.errors
	// 			errs.errors = make([]Error, 0, cap(errs.errors))
	// 		}
	// 		errs.adderrmutex.Unlock()

	// 	}
	// }
}
