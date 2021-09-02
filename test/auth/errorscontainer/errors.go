package errorscontainer

import (
	"time"
)

type ErrorsContainer struct {
	errors chan Error
}

type Error struct {
	Time time.Time
	Err  error
}

type ErrorsHandling interface {
	HandleError(*Error)
}

func NewErrorsContainer(f ErrorsHandling, capacity uint) *ErrorsContainer {
	ch := make(chan Error, capacity)
	go errorslistener(f, ch)
	return &ErrorsContainer{errors: ch}
}

func (c *ErrorsContainer) AddError(err error) {
	c.errors <- Error{Time: time.Now(), Err: err}
}

func errorslistener(f ErrorsHandling, ch chan Error) {
	for err := range ch {
		f.HandleError(&err)
	}
}
