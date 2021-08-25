package errors

import (
	"sync"
	"time"
)

type ErrorsContainer struct {
	capacity int
	iter     int
	errors   []Rec
	mutex    sync.Mutex
}

type Rec struct {
	time time.Time
	err  error
}

func InitErrorsContainer(capacity int) *ErrorsContainer {
	return &ErrorsContainer{capacity: capacity, iter: 0, errors: make([]Rec, 0, capacity), mutex: sync.Mutex{}}
}

func (c *ErrorsContainer) AddError(err error) {
	c.mutex.Lock()
	if len(c.errors) < c.capacity {
		c.errors[c.iter] = Rec{time: time.Now(), err: err}
		c.iter++
	} else {
		// ШО ЭЛСЕ ?
	}
	c.mutex.Unlock()
}

func (c *ErrorsContainer) UnloadErrors() []Rec {
	var res []Rec
	c.mutex.Lock()
	res = c.errors
	c.errors = make([]Rec, 0, c.capacity)
	c.iter = 0
	c.mutex.Unlock()
	return res
}
