package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"project/test/configurator/gopool"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/mailru/easygo/netpoll"
)

type config struct {
	Listen   string
	Settings string
	Servers  []string
}

// короче надо всю эту хуету хранить как то не так
type configurator struct {
	services       map[string]serviceinstance
	servicesStatus map[string]int // TODO: write to memcacshed and add mutex
	hosts          map[string]struct{}
}

type serviceinstance struct {
	addr   string
	wsconn net.Conn
}

// type service struct {
// 	instances []serviceInstance
// 	listeners []net.Conn
// }

// type serviceInstance struct {
// 	addr   string
// 	wsconn net.Conn
// }

func readTomlConfig(path string, conf *config) error {
	if _, err := toml.DecodeFile(path, conf); err != nil {
		return err
	}
	fmt.Println("config: ", conf)
	return nil
}

func nameConn(conn net.Conn) string {
	return conn.LocalAddr().String() + " > " + conn.RemoteAddr().String()
}

func (c *configurator) handle(conn net.Conn, poller netpoll.Poller, pool *gopool.Pool) {

	var servicename string

	u := ws.Upgrader{
		OnRequest: func(uri []byte) error {
			servicename = string(uri[1:])
			if _, ok := c.services[servicename]; !ok {
				fmt.Println("OnReq 403", servicename, "|", string(uri[1:])) //
				return ws.RejectConnectionError(
					ws.RejectionStatus(403),
				)
			}
			return nil
		},
		OnHost: func(host []byte) error {
			return nil
		},
		OnBeforeUpgrade: func() (header ws.HandshakeHeader, err error) {
			s := c.services[servicename]
			if s.addr == "*" {
				if s.addr, err = getfreeaddr(); err != nil {
					ws.RejectConnectionError(
						ws.RejectionStatus(500),
					)
				}
				c.services[servicename] = s
			}
			return ws.HandshakeHeaderHTTP(http.Header{
				"X-listen-here-u-little-shit": []string{c.services[servicename].addr},
			}), nil
		},
	}

	// Zero-copy upgrade to WebSocket connection.
	hs, err := u.Upgrade(conn)
	if err != nil {
		log.Printf("%s: upgrade error: %v", nameConn(conn), err)
		conn.Close()
		return
	}

	log.Printf("%s: established websocket connection: %+v", nameConn(conn), hs)

	// Create netpoll event descriptor for conn.
	// We want to handle only read events of it.
	desc, err := netpoll.HandleRead(conn) // was Must()
	if err != nil {
		fmt.Println("Creating descriptor err:", err)
	}

	// Subscribe to events about conn.
	poller.Start(desc, func(ev netpoll.Event) {
		if ev&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
			// When ReadHup or Hup received, this mean that client has
			// closed at least write end of the connection or connections
			// itself. So we want to stop receive events about such conn.
			poller.Stop(desc)
			fmt.Println("CLOSED", nameConn(conn), "at event", ev)
			return
		}
		// Here we can read some new message from connection.
		// We can not read it right here in callback, because then we will
		// block the poller's inner loop.
		// We do not want to spawn a new goroutine to read single message.
		// But we want to reuse previously spawned goroutine.
		pool.Schedule(func() {
			r := wsutil.NewReader(conn, ws.StateServerSide)
			h, err := r.NextFrame()
			if err != nil {
				fmt.Println("NextFrame err:", err)
				conn.Close()
				poller.Stop(desc)
				return
			}
			if h.OpCode.IsControl() {
				if err = wsutil.ControlFrameHandler(conn, ws.StateServerSide)(h, r); err != nil { // TODO: отказаться от wsutil
					fmt.Println("Control frame handling err:", err, "| Closing", nameConn(conn))
					conn.Close()
					poller.Stop(desc)
					return
				}
				//fmt.Println("Control frame from", nameConn(conn))
				return
			}
			payload := make([]byte, h.Length)
			if _, err = r.Read(payload); err != nil {
				if err == io.EOF {
					err = nil
				} else {
					fmt.Println("Reading payload err:", err)
					conn.Close()
					poller.Stop(desc)
					return
				}
			}

			fmt.Println("PAYLOAD:", string(payload))
		})
	})
}
func main() {
	ln, err := net.Listen("tcp", "localhost:8089")
	if err != nil {
		fmt.Println("Listen err:", err)
	}

	c := &configurator{services: make(map[string]serviceinstance), hosts: map[string]struct{}{}}
	c.hosts["127.0.0.1"] = struct{}{}
	c.services["test.test"] = serviceinstance{addr: "*"}

	// Initialize netpoll instance. We will use it to be noticed about incoming
	// events from listener of user connections.
	poller, err := netpoll.New(nil)
	if err != nil {
		fmt.Println("Init netpoll err:", err)
		return
	}

	exit := make(chan struct{})
	pool := gopool.NewPool(5, 1, 1)

	// Create netpoll descriptor for the listener.
	// We use OneShot here to manually resume events stream when we want to.
	acceptDesc, err := netpoll.HandleListener(ln, netpoll.EventRead|netpoll.EventOneShot)
	if err != nil {
		fmt.Println("Creating accept descriptor err:", err)
		return
	}
	accept := make(chan error, 1)

	poller.Start(acceptDesc, func(e netpoll.Event) {
		// We do not want to accept incoming connection when goroutine pool is
		// busy. So if there are no free goroutines during 1ms we want to
		// cooldown the server and do not receive connection for some short
		// time.
		err := pool.ScheduleTimeout(time.Millisecond, func() {
			conn, err := ln.Accept()
			if err != nil {
				accept <- err
				return
			}

			accept <- nil
			c.handle(conn, poller, pool)
		})
		if err == nil {
			err = <-accept
		}
		if err != nil {
			if err != gopool.ErrScheduleTimeout {
				goto cooldown
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				goto cooldown
			}

			log.Fatalf("accept error: %v", err)

		cooldown:
			delay := 5 * time.Millisecond
			log.Printf("accept error: %v; retrying in %s", err, delay)
			time.Sleep(delay)
		}

		poller.Resume(acceptDesc)
	})
	<-exit
}

func getfreeaddr() (string, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return "", err
	}
	defer l.Close()
	return l.Addr().String(), nil
}

// for {
// 	conn, err := ln.Accept()
// 	if err != nil {
// 		fmt.Println(1, err)
// 		return
// 	}
// 	_, err = ws.Upgrade(conn)
// 	if err != nil {
// 		fmt.Println(2, err)
// 		return
// 	}
// 	go func() {
// 		defer conn.Close()

// 		for {
// 			header, err := ws.ReadHeader(conn)
// 			if err != nil {
// 				fmt.Println(3, err)
// 				return
// 			}

// 			payload := make([]byte, header.Length)
// 			_, err = io.ReadFull(conn, payload)
// 			if err != nil {
// 				fmt.Println(4, err)
// 				return
// 			}

// 			if header.Masked {
// 				ws.Cipher(payload, header.Mask, 0)
// 			}
// 			fmt.Println("PAYLOAD:", string(payload))
// 			// Reset the Masked flag, server frames must not be masked as
// 			// RFC6455 says.
// 			header.Masked = false

// 			// if err := ws.WriteHeader(conn, header); err != nil {
// 			// 	fmt.Println(5, err)
// 			// 	return
// 			// }
// 			if header.OpCode == ws.OpClose {
// 				fmt.Println("CLOSED")
// 				return
// 			}
// 			// if _, err := conn.Write(payload); err != nil {
// 			// 	fmt.Println(6, err)
// 			// 	return
// 			// }
// 			fr := ws.NewFrame(ws.OpText, true, []byte("hi mark"))
// 			if err = ws.WriteFrame(conn, fr); err != nil {
// 				fmt.Println(err)
// 			}
// 		}
// 	}()
// }

// ctx, cancel := httpservice.CreateContextWithInterruptSignal()

// l, err := logscontainer.NewLogsContainer(ctx, flushers.NewConsoleFlusher("CNFG"), 1, time.Second, 1)
// if err != nil {
// 	fmt.Println("logs init err:", err)
// 	return
// }
// defer func() {
// 	cancel()
// 	<-l.Done
// }()

// configurator, err := NewConfigurator(conf.Settings, conf.Servers)
// if err != nil {
// 	fmt.Println(err)
// 	return
// }
// http.HandleFunc("/", configurator.handler)
// if err := http.ListenAndServe(conf.Listen, nil); err != nil {
// 	fmt.Println(err)
// }

//}

// func (configurator *Configurator) handler(w http.ResponseWriter, r *http.Request) {
// 	ws, err := configurator.upgrader.Upgrade(w, r, nil)
// 	if err != nil {
// 		fmt.Println("Upgrade err:", err)
// 	}
// 	fmt.Println("Connected")
// 	reader(ws)
// }

// func reader(conn *websocket.Conn) {
// 	for {
// 		// read in a message
// 		messageType, p, err := conn.ReadMessage()
// 		if err != nil {
// 			log.Println(err)
// 			return
// 		}
// 		// print out that message for clarity
// 		fmt.Println(string(p))

// 		if err := conn.WriteMessage(messageType, p); err != nil {
// 			log.Println(err)
// 			return
// 		}

// 	}
// }
