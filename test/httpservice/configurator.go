package httpservice

import (
	"context"
	"errors"
	"net"
	"project/test/connector"
	"project/test/types"
	"strings"
	"time"

	"github.com/big-larry/suckutils"
)

type configurator struct {
	conn            *connector.EpollReConnector
	thisServiceName ServiceName

	l types.Logger
}

func NewConfigurator(ctx context.Context, l types.Logger, configuratoraddr string, thisServiceName ServiceName, reconnectTimeout time.Duration) (*configurator, error) {

	conn, err := net.Dial(configuratoraddr[:strings.Index(configuratoraddr, ":")], configuratoraddr[strings.Index(configuratoraddr, ":")+1:])
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(connector.FormatBasicMessage([]byte(thisServiceName))); err != nil {
		return nil, err
	}
	time.Sleep(time.Second)
	buf := make([]byte, 5)
	conn.SetReadDeadline(time.Now().Add(time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		conn.Close()
		return nil, errors.New(suckutils.ConcatTwo("err reading configurator's approving, err: ", err.Error()))
	}
	if n != 5 || buf[4] != byte(types.OperationCodeOK) {
		conn.Close()
		return nil, errors.New("configurator's approving format not supported or weird")
	}
	c := &configurator{thisServiceName: thisServiceName}

	connector.InitReconnection(ctx, reconnectTimeout, 1, 1)
	connector.SetupEpoll(func(e error) { panic(e) })

	if c.conn, err = connector.NewEpollReConnector(conn, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *configurator) NewMessage() connector.MessageReader {
	return connector.NewBasicMessage()
}

func (c *configurator) Handle(message connector.MessageReader) error {
	return errors.New("TODO")
}

func (c *configurator) HandleClose(reason error) {
	c.l.Warning("Configurator", suckutils.ConcatTwo("conn closed, reason err: ", reason.Error()))
	// в суспенд не уходим, пока у нас есть паблишеры - нам пофиг
}
