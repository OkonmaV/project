package connector

import (
	"errors"
	"time"
)

var ErrWeirdData error = errors.New("weird data")
var ErrEmptyPayload error = errors.New("empty payload")
var ErrClosedConnector error = errors.New("closed connector")
var ErrNilConn error = errors.New("conn is nil")
var ErrNilGopool error = errors.New("gopool is nil, setup gopool first")

// for passing net.Conn
type ConnReader interface {
	Read([]byte) (int, error)
	SetReadDeadline(time.Time) error
}

// for user's implementation
type MessageReader interface {
	Read(ConnReader) error
}

// for user's implementation
type MessageHandler interface {
	NewMessage() MessageReader
	Handle(MessageReader) error
	HandleClose(error)
}

// implemented by connector
type MessageSender interface {
	Send([]byte) error
}

// implemented by connector
type Informer interface {
	GetRemoteAddr() (string, string)
	IsClosed() bool
}

// implemented by connector
type ConnCloser interface {
	Close(error)
}
