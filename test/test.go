package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"time"

	"strconv"
)

type helloer interface {
	Hello(string)
}

type A struct {
	Str string
}
type B struct {
	str string
}

func (a *A) Hello(s string) {
	a.Str = s
	println("Helloed " + s)
}

func Create[M any, PT interface {
	helloer
	*M
}](n int) (out []*M) {
	for i := 0; i < n; i++ {
		v := PT(new(M))
		v.Hello(strconv.Itoa(i))
		out = append(out, (*M)(v))
	}
	return
}

type MessageHandler[T helloer] interface {
	HHandle(T) error
	HHandleClose(reason error)
}

func NewEpollConnector[Tmessage any,
	PTmessage interface {
		helloer
		*Tmessage
	}, TT MessageHandler[PTmessage]](messagehandler TT) {
	msg := PTmessage(new(Tmessage))
	msg.Hello("hey")

	messagehandler.HHandle(msg)
	fmt.Println(messagehandler)
}

func main() {
	var conn1 net.Conn
	go func() {
		println("--- grt started")
		ln, err := net.Listen("tcp", "127.0.0.1:8099")
		if err != nil {
			panic(err)
		}
		conn1, err = ln.Accept()
		if err != nil {
			panic(err)
		}
		println("--- conn accepted")
		if _, err = conn1.Write([]byte("test1")); err != nil {
			panic(err.Error())
		}
		println("--- writed test1")
	}()
	time.Sleep(time.Second)
	conn2, err := net.Dial("tcp", "127.0.0.1:8099")
	if err != nil {
		panic(err)
	}
	println("+++ dialed")
	time.Sleep(time.Second)
	buf := make([]byte, 10)

	go func() {
		time.Sleep(time.Second)
		conn2.Close()
	}()

	n, err := io.ReadFull(conn2, buf)
	if err != nil {
		er, ok := err.(*net.OpError)
		println(fmt.Sprint(errors.Is(er.Err, net.ErrClosed), ok))
		println(fmt.Sprint(reflect.TypeOf(err)))
		panic(err)
	}
	println("+++ readed", strconv.Itoa(n), "bytes, buf:", fmt.Sprint(buf), "=", string(buf))

	// buf = make([]byte, 1)
	// conn1.Close()
	// n, err = conn2.Read(buf)
	// if err != nil {
	// 	panic(err)
	// }
	// println("+++ readed", strconv.Itoa(n), "bytes, buf:", fmt.Sprint(buf), "=", string(buf))

	// go func() {
	// 	println("--- grt started")
	// 	ln, err := net.Listen("tcp", "127.0.0.1:8099")
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	c, err := ln.Accept()
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	println("--- conn accepted")
	// 	if _, err = c.Write([]byte("test1")); err != nil {
	// 		panic(err.Error())
	// 	}
	// 	println("--- writed test1, sleep")
	// 	time.Sleep(time.Second)
	// 	c.Close()
	// 	println("--- conn closed")
	// }()
	// time.Sleep(time.Second)
	// b := &B{"beforetest1"}
	// connector.SetupEpoll(nil)
	// connector.SetupPoolHandling(dynamicworkerspool.NewPool(2, 5, time.Second))
	// conn, err := net.Dial("tcp", "127.0.0.1:8099")
	// if err != nil {
	// 	panic(err)
	// }

	// rc, err := rp.NewEpollReConnector(conn, b, nil, func() error {
	// 	println("=== reconnected")
	// 	return nil
	// }) //connector.NewEpollConnector[mesag](conn, b)
	// if err != nil {
	// 	panic(err)
	// }
	// if err = rc.StartServing(); err != nil {
	// 	panic(err)
	// }
	// println("+++ reconnector created, sleep")
	// time.Sleep(time.Second * 2)
	// println("+++ wake up")

	// println("+++ b.str now is", b.str)
	// time.Sleep(time.Hour)

	//crt := Create[A](2)
	//fmt.Println(crt[1].Str)
}

func (b *B) Handle(m *mesag) error {
	println("=== new message:", m.str, ", b.str was:", b.str)
	b.str = m.str
	return nil
}

func (*B) HandleClose(r error) {
	println("=== conn closed:", r.Error())
}

type mesag struct {
	str string
}

func (m *mesag) Read(conn net.Conn) error {
	b := make([]byte, 5)
	_, err := conn.Read(b)
	if err != nil {
		return err
	}
	m.str = string(b)
	return nil
}
