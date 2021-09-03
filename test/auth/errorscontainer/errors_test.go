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

type ErrorsFlusher interface {
	Flush([]Error) error
}

func NewErrorsContainer(ctx context.Context, f ErrorsFlusher, capacity int, flushperiod time.Duration, minflushinglen int) (*ErrorsContainer, error) {

	if capacity < 1 {
		return nil, errors.New("capacity < 1")
	}
	if minflushinglen < 1 {
		return nil, errors.New("minflushinglen > capacity")
	}
	if minflushinglen < 1 || minflushinglen > capacity {
		return nil, errors.New("minflushinglen < 1")
	}

	container := &ErrorsContainer{
		errors:      make([]Error, 0, capacity),
		adderrmutex: sync.Mutex{},
		flushmutex:  sync.Mutex{},
	}

	go errorslistener(ctx, f, container, flushperiod, minflushinglen)

	return container, nil
}
func (errs *ErrorsContainer) AddError(err error) {
	errs.adderrmutex.Lock()
	defer errs.adderrmutex.Unlock()
	if err == nil {
		return
	}
	// А где проверка на длину массива и флуш!? У тебя получается флуш только по тику... М вчера говорили о том, что флуш может быть как по тику, так и при большом количестве ошибок...
	errs.errors = append(errs.errors, Error{Time: time.Now(), Err: err})
}

// Все что ниже обсудим
func errorslistener(ctx context.Context, f ErrorsFlusher, errs *ErrorsContainer, flushperiod time.Duration, minflushinglen int) {

	ticker := time.NewTicker(flushperiod)
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

func initflush(wg *sync.WaitGroup, errs *ErrorsContainer, f ErrorsFlusher, flusherrs []Error) {
	defer wg.Done()
	errs.flushmutex.Lock()
	if err := f.Flush(flusherrs); err != nil {
		errs.AddError(err) // потенциальная блокировка?
	}
	errs.flushmutex.Unlock()
}
