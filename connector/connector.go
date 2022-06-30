package connector

import (
	"errors"
	"net"
)

var ErrWeirdData error = errors.New("weird data")
var ErrEmptyPayload error = errors.New("empty payload")
var ErrClosedConnector error = errors.New("closed connector")
var ErrNilConn error = errors.New("conn is nil")
var ErrNilGopool error = errors.New("gopool is nil, setup gopool first")
var ErrReadTimeout error = errors.New("read timeout")

// for user's implementation
type MessageReader interface {
	Read(net.Conn) error
}

// for user's implementation
type MessageHandler interface {
	NewMessage() MessageReader
	Handle(message interface{}) error
	HandleClose(error)
}

type Conn interface {
	StartServing() error
	ClearFromCache()
	Informer
	Closer
	Sender
}

type ReConn interface {
	Conn
	ReconnectedItself(net.Conn) error
	IsReconnectStopped() bool
	CancelReconnect()
}

// implemented by connector
type Sender interface {
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
