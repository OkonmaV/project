package errorscontainer_test

import (
	"context"
	"errors"
	"sync"
	"time"
)

type ErrorsContainer struct {
	errors      []Error
	adderrmutex sync.Mutex
	flushmutex  sync.Mutex
}

type Error struct {
	Time time.Time
	Err  error
}

type ErrorsHandling interface {
	Flush([]Error) error
}

func NewErrorsContainer(ctx context.Context, f ErrorsHandling, capacity uint, ticktime time.Duration, errpackcapacity int) *ErrorsContainer {
	errs := make([]Error, 0, capacity)
	foo := &ErrorsContainer{errors: errs, adderrmutex: sync.Mutex{}, flushmutex: sync.Mutex{}}
	go errorslistener(ctx, f, foo, ticktime, errpackcapacity)
	return foo
}
func (errs *ErrorsContainer) AddError(err error) {
	errs.adderrmutex.Lock()
	errs.errors = append(errs.errors, Error{Time: time.Now(), Err: err})
	errs.adderrmutex.Unlock()
}

func errorslistener(ctx context.Context, f ErrorsHandling, errs *ErrorsContainer, ticktime time.Duration, minflushinglen int) {
	if minflushinglen == 0 || minflushinglen > cap(errs.errors) {
		minflushinglen = cap(errs.errors)
		errs.AddError(errors.New("bad minflushinglen, changed to cap(errs)"))
	}
	ticker := time.NewTicker(ticktime)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			errs.adderrmutex.Lock()
			if len(errs.errors) >= minflushinglen {
				flushingerrs := make([]Error, len(errs.errors)) //o/ flushniggers
				copy(flushingerrs, errs.errors)
				errs.errors = errs.errors[:0] //

				go func() {
					errs.flushmutex.Lock()
					if err := f.Flush(flushingerrs); err != nil {
						errs.AddError(err) // потенциальная блокировка?
					}
					errs.flushmutex.Unlock()
				}()

			}
			errs.adderrmutex.Unlock()

		}
	}
}
