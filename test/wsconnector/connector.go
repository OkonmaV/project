package wsconnector

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

type StatusCode int

type UpgradeCreator interface {
	CreateNewConnInfo() WsHandler
}

// for user's implementation
type WsHandler interface {
	UpgradeReqChecker
	SetSender(Sender)
	Handle(message []byte) error
	HandleClose(error)
}

// for user's implementation
// for ckecking headers while reading it in ws.Upgrade()
type UpgradeReqChecker interface {
	// 200 = no err
	CheckPath(path []byte) StatusCode
	// 200 = no err
	CheckHost(host []byte) StatusCode
	// 200 = no err
	CheckHeader(key []byte, value []byte) StatusCode
	// 200 = no err
	CheckBeforeUpgrade() StatusCode
}

type Conn interface {
	StartServing() error
	ClearFromCache()
	Informer
	Closer
	Sender
}

// implemented by connector
type Sender interface {
	Send(payload []byte) error
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
