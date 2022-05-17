package epolllistener

import (
	"errors"
	"net"
)

var ErrWeirdData error = errors.New("weird data")
var ErrReadTimeout error = errors.New("read timeout")
var ErrAuthorizationDenied error = errors.New("authorization denied")

// for user's implementation
type ListenerHandler interface {
	HandleNewConn(net.Conn)
	AcceptError(error) // all errs during accept and so on, e.g. auth denied, accept errs
}

// implemented by listener
type Informer interface {
	RemoteAddr() net.Addr
	IsClosed() bool
}

// implemented by listener
type Closer interface {
	Close(error)
}
