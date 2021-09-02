package errorscontainer_test

import (
	"context"
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

func NewErrorsContainer(ctx context.Context, f ErrorsHandling, capacity int, ticktime time.Duration, minflushinglen int) *ErrorsContainer {

	if capacity < 1 {
		capacity = 10
	}
	if minflushinglen < 1 || minflushinglen > capacity {
		minflushinglen = capacity
	}

	errs := make([]Error, 0, capacity)
	container := &ErrorsContainer{errors: errs, adderrmutex: sync.Mutex{}, flushmutex: sync.Mutex{}}

	go errorslistener(ctx, f, container, ticktime, minflushinglen)

	return container
}
func (errs *ErrorsContainer) AddError(err error) {
	errs.adderrmutex.Lock()
	errs.errors = append(errs.errors, Error{Time: time.Now(), Err: err})
	errs.adderrmutex.Unlock()
}

func errorslistener(ctx context.Context, f ErrorsHandling, errs *ErrorsContainer, ticktime time.Duration, minflushinglen int) {

	ticker := time.NewTicker(ticktime)
	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():

			wg.Wait()                               // потому что флуши могут возвращать ошибки
			if len(errs.errors) >= minflushinglen { // TODO: в зависимости от алгоритма защиты от переполнения возможно дописать
				wg.Add(1)
				go initflush(&wg, errs, f, errs.errors) // из-за возможности возварщения флушем ошибки есть вероятность блокировки, либо предусмотреть возможность на эту последнюю ошибку положить
			}
			wg.Wait()
			return
		case <-ticker.C:
			errs.adderrmutex.Lock()

			if len(errs.errors) >= minflushinglen {
				flusherrs := make([]Error, len(errs.errors))
				copy(flusherrs, errs.errors)
				errs.errors = errs.errors[:0] //
				wg.Add(1)
				go initflush(&wg, errs, f, flusherrs)
			}

			errs.adderrmutex.Unlock()
		}
		// есть идея сделать канал, в который adderror будет пихать сигнал, шо массив заполнен (но звучит кривовато), сейчас защиты от переполнения нету
	}
}

func initflush(wg *sync.WaitGroup, errs *ErrorsContainer, f ErrorsHandling, flusherrs []Error) {
	defer wg.Done()
	errs.flushmutex.Lock()
	if err := f.Flush(flusherrs); err != nil {
		errs.AddError(err) // потенциальная блокировка?
	}
	errs.flushmutex.Unlock()
}
