package customlistener

import (
	"errors"
	"net"
	"time"
)

var ErrWeirdData error = errors.New("weird data")
var ErrReadTimeout error = errors.New("read timeout")
var ErrAuthorizationDenied error = errors.New("authorization denied")

// for passing net.Conn
type ConnReader interface {
	Read([]byte) (int, error)
	SetReadDeadline(time.Time) error
	RemoteAddr() net.Addr
}

// for user's implementation
type ListenerHandler interface {
	AuthorizeConn(ConnReader) error
	HandleNewConn(net.Conn) error // called only for succesfully authorized conn
	HandleError(error)            // all errs during accept and so on, e.g. auth denied, accept errs
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
