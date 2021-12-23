package connector

import (
	"errors"
	"net"
	"time"
)

var ErrWeirdData error = errors.New("weird data")
var ErrEmptyPayload error = errors.New("empty payload")
var ErrClosedConnector error = errors.New("closed connector")
var ErrNilConn error = errors.New("conn is nil")
var ErrNilGopool error = errors.New("gopool is nil, setup gopool first")
var ErrReadTimeout error = errors.New("read timeout")

// for passing net.Conn
type ConnReader interface {
	Read([]byte) (int, error)
	SetReadDeadline(time.Time) error
	RemoteAddr() net.Addr
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

type Conn interface {
	StartServing() error
	MessageHandler
	Informer
	Closer
}

// implemented by connector
type MessageSender interface {
	Send([]byte) error
}

// implemented by connector
type Informer interface {
	RemoteAddr() net.Addr
	IsClosed() bool
}

// implemented by connector
type Closer interface {
	Close(error)
}
