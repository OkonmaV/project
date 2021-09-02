package errorscontainer_test

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
	HandleErrors([]*Error)
}

func NewErrorsContainer(f ErrorsHandling, capacity uint, ticktime time.Duration, errpackcapacity uint) *ErrorsContainer {
	ch := make(chan Error, capacity)
	go errorslistener(f, ch, ticktime, errpackcapacity)
	return &ErrorsContainer{errors: ch}
}
func (c *ErrorsContainer) AddError(err error) {
	c.errors <- Error{Time: time.Now(), Err: err}
}

func errorslistener(f ErrorsHandling, ch chan Error, ticktime time.Duration, errpackcapacity uint) {
	ticker := time.NewTicker(ticktime)
	errpack := make([]*Error, 0, errpackcapacity)
	for range ticker.C {
		for err := range ch {
			if len(errpack) < int(errpackcapacity) {
				errpack = append(errpack, &err)
			} else {
				f.HandleErrors(errpack) //если посыпятся ошибки и хэндлер тоже сдохнет, то блокировка из-за аддеррор // либо хэндлер по другому должен ловить ошибки, шо неоч
				errpack = errpack[:0]
				break // максимум один флуш за тик, иначе риск петли при лавине ошибок
			}
			// break ??
		}
	}
}
