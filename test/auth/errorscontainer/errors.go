package errorscontainer

import (
	"sync"
	"time"
)

type ErrorsContainer struct {
	capacity int
	iter     int
	errors   []rec
	mutex    sync.Mutex
}

type rec struct {
	time time.Time
	err  error
}

func NewErrorsContainer(capacity int) *ErrorsContainer {
	return &ErrorsContainer{capacity: capacity, iter: 0, errors: make([]rec, 0, capacity), mutex: sync.Mutex{}}
}

func (c *ErrorsContainer) AddError(err error) {
	c.mutex.Lock()
	if len(c.errors) < c.capacity {
		c.errors = append(c.errors, rec{time: time.Now(), err: err})
		c.iter++
	} else {
		// ШО ЭЛСЕ ?
	}
	c.mutex.Unlock()
}

func (c *ErrorsContainer) UnloadErrors() []rec {
	var res []rec
	c.mutex.Lock()
	res = c.errors
	c.errors = make([]rec, 0, c.capacity)
	c.iter = 0
	c.mutex.Unlock()
	return res
}
