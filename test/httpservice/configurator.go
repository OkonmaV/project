package httpservice

import (
	"context"
	"errors"
	"fmt"
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

	publishers *publishers
	listener   *listener
	servStatus *serviceStatus

	terminationByConfigurator chan struct{}
	l                         types.Logger
}

func newConfigurator(ctx context.Context, l types.Logger, servStatus *serviceStatus, pubs *publishers, listener *listener, configuratoraddr string, thisServiceName ServiceName, reconnectTimeout time.Duration) *configurator {

	c := &configurator{
		thisServiceName: thisServiceName,
		l:               l, servStatus: servStatus,
		publishers:                pubs,
		terminationByConfigurator: make(chan struct{}, 1)}

	connector.InitReconnection(ctx, reconnectTimeout, 1, 1)

	go func() {
		for {
			conn, err := net.Dial((configuratoraddr)[:strings.Index(configuratoraddr, ":")], (configuratoraddr)[strings.Index(configuratoraddr, ":")+1:])
			if err != nil {
				l.Error("Dial", err)
				goto timeout
			}

			if err = c.handshake(conn); err != nil {
				conn.Close()
				l.Error("handshake", err)
				goto timeout
			}
			if c.conn, err = connector.NewEpollReConnector(conn, c, c.handshake, c.afterConnProc, "", ""); err != nil {
				l.Error("NewEpollReConnector", err)
				goto timeout
			}
			if err = c.conn.StartServing(); err != nil {
				c.conn.ClearFromCache()
				l.Error("StartServing", err)
				goto timeout
			}
			if err = c.afterConnProc(); err != nil {
				c.conn.Close(err)
				l.Error("afterConnProc", err)
				goto timeout
			}
			break
		timeout:
			l.Debug("First connection", "failed, timeout")
			time.Sleep(reconnectTimeout)
		}
	}()

	return c
}

func (c *configurator) handshake(conn net.Conn) error {
	println("message: ", string(connector.FormatBasicMessage([]byte(c.thisServiceName))))
	if _, err := conn.Write(connector.FormatBasicMessage([]byte(c.thisServiceName))); err != nil {
		return err
	}
	time.Sleep(time.Second)
	buf := make([]byte, 5)
	conn.SetReadDeadline(time.Now().Add(time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		return errors.New(suckutils.ConcatTwo("err reading configurator's approving, err: ", err.Error()))
	}
	if n == 5 {
		if buf[4] == byte(types.OperationCodeOK) {
			return nil
		} else if buf[4] == byte(types.OperationCodeNOTOK) {
			go c.conn.CancelReconnect() // горутина пушто этот хэндшейк под залоченным мьютексом выполняется
			c.terminationByConfigurator <- struct{}{}
			return errors.New("configurator do not approve this service")
		}
	}
	return errors.New("configurator's approving format not supported or weird")
}

func (c *configurator) afterConnProc() error {
	myStatus := byte(types.StatusSuspended)
	if c.servStatus.onAir() {
		myStatus = byte(types.StatusOn)
	}
	if err := c.conn.Send(connector.FormatBasicMessage([]byte{myStatus})); err != nil {
		return err
	}

	if c.publishers != nil {
		pubnames := c.publishers.GetAllPubNames()
		if len(pubnames) != 0 {
			message := append(make([]byte, 0, len(pubnames)*15), byte(types.OperationCodeSubscribeToServices))
			for _, pub_name := range pubnames {
				pub_name_byte := []byte(pub_name)
				message = append(append(message, byte(len(pub_name_byte))), pub_name_byte...)
			}
			if err := c.conn.Send(connector.FormatBasicMessage(message)); err != nil {
				return err
			}
		}
	}

	if err := c.conn.Send(connector.FormatBasicMessage([]byte{byte(types.OperationCodeGiveMeOuterAddr)})); err != nil {
		return err
	}
	return nil
}

func (c *configurator) send(message []byte) error {
	if c == nil {
		return errors.New("nil configurator")
	}
	if c.conn == nil {
		return connector.ErrNilConn
	}
	if c.conn.IsClosed() {
		return connector.ErrClosedConnector
	}
	fmt.Println("send", message, "|||", string(message))
	return c.conn.Send(message)
}

func (c *configurator) onSuspend(reason string) {
	c.l.Warning("OwnStatus", suckutils.ConcatTwo("suspended, reason: ", reason))
	if c.conn != nil && !c.conn.IsClosed() {
		if err := c.conn.Send(connector.FormatBasicMessage([]byte{byte(types.OperationCodeMyStatusChanged), byte(types.StatusSuspended)})); err != nil {
			c.l.Error("Send", err)
		}
	}
}

func (c *configurator) onUnSuspend() {
	c.l.Warning("OwnStatus", "unsuspended")
	if c.conn != nil && !c.conn.IsClosed() {
		if err := c.conn.Send(connector.FormatBasicMessage([]byte{byte(types.OperationCodeMyStatusChanged), byte(types.StatusOn)})); err != nil {
			c.l.Error("Send", err)
		}
	}
}

func (c *configurator) NewMessage() connector.MessageReader {
	return connector.NewBasicMessage()
}

func (c *configurator) Handle(message connector.MessageReader) error {
	payload := message.(connector.BasicMessage).Payload
	if len(payload) == 0 {
		return connector.ErrEmptyPayload
	}
	switch types.OperationCode(payload[0]) {
	case types.OperationCodePing:
		return nil
	case types.OperationCodeMyStatusChanged:
		return nil
	case types.OperationCodeImSupended:
		return nil
	case types.OperationCodeSetOutsideAddr:
		if len(payload) < 2 {
			return connector.ErrWeirdData
		}
		if len(payload) < 2+int(payload[1]) {
			return connector.ErrWeirdData
		}
		if netw, addr, err := types.UnformatAddress(payload[2 : 3+int(payload[1])]); err != nil {
			return err
		} else {
			if netw == types.NetProtocolNil {
				c.listener.stop()
				c.servStatus.setListenerStatus(true)
			}
			if cur_netw, cur_addr := c.listener.Addr(); cur_addr == addr && cur_netw == netw.String() {
				return nil
			}
			var err error
			for i := 0; i < 3; i++ {
				if err = c.listener.listen(netw.String(), addr); err != nil {
					c.listener.l.Error("listen", err)
					time.Sleep(time.Second)
				} else {
					return nil
				}
			}
			return err
		}
	case types.OperationCodeUpdatePubs:
		updates := types.SeparatePayload(payload[1:])
		if len(updates) != 0 {
			for _, update := range updates {
				pubname, raw_addr, status, err := types.UnformatOpcodeUpdatePubMessage(update)
				if err != nil {
					return err
				}
				netw, addr, err := types.UnformatAddress(raw_addr)
				if err != nil {
					c.l.Error("Handle/OperationCodeUpdatePubs/UnformatAddress", err)
					return connector.ErrWeirdData
				}
				if netw == types.NetProtocolNonlocalUnix {
					continue // TODO:
				}

				c.publishers.update(ServiceName(pubname), netw.String(), addr, status)
			}
		} else {
			return connector.ErrWeirdData
		}
	}
	return connector.ErrWeirdData
}

func (c *configurator) HandleClose(reason error) {
	c.l.Warning("Configurator", suckutils.ConcatTwo("conn closed, reason err: ", reason.Error()))
	// в суспенд не уходим, пока у нас есть паблишеры - нам пофиг
}
