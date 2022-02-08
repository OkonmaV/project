package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"project/test/connector"
	"project/test/epolllistener"
	"project/test/types"
	"strconv"
	"strings"
	"time"
)

type conf struct {
	conn *connector.EpollReConnector
}

func (c *conf) NewMessage() connector.MessageReader {
	return connector.NewBasicMessage()
}

func bytetostring(arr []byte) string {
	p := make([]string, 0, len(arr))
	for i := 0; i < len(arr); i++ {
		k := strconv.Itoa(int(arr[i]))
		p = append(p, k)
	}
	return "[ " + strings.Join(p, " ") + " ]"
}

func (c *conf) Handle(message connector.MessageReader) error {
	defer println("\n")
	payload := message.(*connector.BasicMessage).Payload

	println("client: new message recieved: ", bytetostring(payload))
	return nil
}

func (c *conf) HandleClose(reason error) {
	defer println("\n")
	println("client: handleclose, reason: ", reason.Error())
}

func (c *conf) handshake(conn net.Conn) error {
	defer println("\n")
	msg := connector.FormatBasicMessage([]byte("client"))
	println("client: sending handshake message: ", bytetostring(msg))
	if _, err := conn.Write(msg); err != nil {
		println("client: write handshake err: ", err.Error())
		return err
	}
	time.Sleep(time.Second)
	buf := make([]byte, 5)
	conn.SetReadDeadline(time.Now().Add(time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		println("client: reading handshake responce err: ", err.Error())
		return err
	}
	if n == 5 {
		if buf[4] == byte(types.OperationCodeOK) {
			println("client: response to handshake OK")
			return nil
		} else if buf[4] == byte(types.OperationCodeNOTOK) {
			println("client: response to handshake NOT OK")
			return errors.New("configurator do not approve this service")
		}
	}
	return errors.New("configurator's approving format not supported or weird")
}

func (c *conf) afterConnProc() error {
	defer println("\n")
	pubnames := []string{"publisher1", "publisher2"}
	message := append(make([]byte, 0, len(pubnames)*15), byte(types.OperationCodeSubscribeToServices))
	for _, pub_name := range pubnames {
		pub_name_byte := []byte(pub_name)
		message = append(append(message, byte(len(pub_name_byte))), pub_name_byte...)
	}
	message = connector.FormatBasicMessage(message)
	println("client: sending pubs request: ", bytetostring(message))
	if err := c.conn.Send(message); err != nil {
		println("client: sending pubs request err: ", err.Error())
		return err
	}
	return nil
}

func main() {
	p, err := strconv.Atoi(string([]byte{4, 56, 48, 57, 57}))
	fmt.Println(p, err, string([]byte{4, 56, 48, 57, 57}))
	return

	connector.InitReconnection(context.Background(), time.Second*3, 1, 1)
	connector.SetupEpoll(nil)
	c := &conf{}
	go func() {
		for {
			conn, err := net.Dial("tcp", "127.0.0.1:9090")
			if err != nil {
				println("client: conn dial err: ", err.Error())
				goto timeout
			}

			if err = c.handshake(conn); err != nil {
				conn.Close()
				println("client: conn handshake err: ", err.Error())
				goto timeout
			}
			if c.conn, err = connector.NewEpollReConnector(conn, c, c.handshake, c.afterConnProc, "", ""); err != nil {
				println("client: conn newepollreconnector err: ", err.Error())
				goto timeout
			}
			if err = c.conn.StartServing(); err != nil {
				println("client: conn startserving err: ", err.Error())
				c.conn.ClearFromCache()
				goto timeout
			}
			if err = c.afterConnProc(); err != nil {
				c.conn.Close(err)
				println("client: conn afterconnproc err: ", err.Error())
				goto timeout
			}
			break
		timeout:
			println("client: conn failed, timeout 3s\n")
			time.Sleep(time.Second * 3)
		}
	}()

	newListener("tcp", "127.0.0.1:9090")
	time.Sleep(time.Hour)
}

type listener_info struct{}

func newListener(network, address string) {
	defer println("\n")
	lninfo := &listener_info{}
	ln, err := epolllistener.EpollListen(network, address, lninfo)
	if err != nil {
		panic("conf: epolllisten err: " + err.Error())
	}
	if err = ln.StartServing(); err != nil {
		panic("conf: startserving err: " + err.Error())
	}
	println("conf: start listening at ", address)
}

type service struct {
	addr string
	con  connector.Conn
}

func (lninfo *listener_info) HandleNewConn(conn net.Conn) {
	defer println("\n")
	println("conf: handlenewconn from ", conn.RemoteAddr().String())
	conn.SetReadDeadline(time.Now().Add(time.Second * 2))
	buf := make([]byte, 4)
	_, err := conn.Read(buf)
	if err != nil {
		println("conf: read 4bytes err: ", err.Error())
		conn.Close()
		return
	}

	buf = make([]byte, binary.LittleEndian.Uint32(buf))
	_, err = conn.Read(buf)
	if err != nil {
		println("conf: read name err: ", err.Error())
		conn.Close()
		return
	}
	name := string(buf)
	println("conf: readed name: ", name)

	service := &service{addr: conn.RemoteAddr().String()}

	var con *connector.EpollConnector
	if con, err = connector.NewEpollConnector(conn, service); err != nil {
		println("conf: newepollconnector err: ", err.Error())
		conn.Close()
		return
	}
	if err = con.StartServing(); err != nil {
		println("conf: startserving err: ", err.Error())
		con.ClearFromCache()
		conn.Close()
		return
	}
	service.con = con
	msg := connector.FormatBasicMessage([]byte{byte(types.OperationCodeOK)})
	println("conf: sending OK: ", bytetostring(msg))
	if err := con.Send(msg); err != nil {
		println("client: sending OK err: ", err.Error())
		con.Close(err)
		return
	}
}

func (lninfo *listener_info) AcceptError(err error) {
	defer println("\n")
	println("conf: accept err: ", err.Error())
}

func (s *service) NewMessage() connector.MessageReader {
	return connector.NewBasicMessage()
}

func (s *service) Handle(message connector.MessageReader) error {
	defer println("\n")
	payload := message.(*connector.BasicMessage).Payload
	println("conf: [", s.addr, "] recieved new message: ", bytetostring(payload))
	if len(payload) == 0 {
		println("conf: [", s.addr, "] err: payload zero len")
		return nil //connector.ErrWeirdData
	}
	switch types.OperationCode(payload[0]) {
	case types.OperationCodeSubscribeToServices:
		println("conf: [", s.addr, "] opcode pubsrequest")
		raw_pubnames := types.SeparatePayload(payload[1:])
		if raw_pubnames == nil {
			println("conf: [", s.addr, "] err: rawpubnames zero len after separation")
			return connector.ErrWeirdData
		}
		pubnames := make([]string, 0, len(raw_pubnames))
		for _, raw_pubname := range raw_pubnames {
			if len(raw_pubname) == 0 {
				println("conf: [", s.addr, "] err: rawpubnames zero len of one of")
				return connector.ErrWeirdData
			}
			pubnames = append(pubnames, string(raw_pubname))
		}
		println("conf: [", s.addr, "] pubs requested: ", strings.Join(pubnames, ", "))
		msg := connector.FormatBasicMessage([]byte{byte(99)})
		println("client: sending response: ", bytetostring(msg))
		if err := s.con.Send(msg); err != nil {
			println("client: sending OK err: ", err.Error())
			return err
		}
		return nil
	default:
		println("conf: [", s.addr, "] err: unknown opcode")
		return connector.ErrWeirdData
	}
	return nil
}

func (s *service) HandleClose(reason error) {
	defer println("\n")
	println("conf: [", s.addr, "] handleclose, reason: ", reason.Error())
}
