package errorscontainer

type ErrorsContainer struct {
	errors chan error
}

type ErrorsHandling interface {
	Flush(error)
}

func NewErrorsContainer(f ErrorsHandling, capacity uint) *ErrorsContainer {
	ch := make(chan error, capacity)
	go errorslistener(f, ch)
	return &ErrorsContainer{errors: ch}
}

func (c *ErrorsContainer) AddError(err error) {
	c.errors <- err
}

func errorslistener(f ErrorsHandling, ch chan error) {
	for err := range ch {
		f.HandleError(err)
	}
}
