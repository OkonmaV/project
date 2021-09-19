package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"project/test/configurator/gopool"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/mailru/easygo/netpoll"
)

type config struct {
	Listen   string
	Settings string
	Hosts    []string
}

// короче надо всю эту хуету хранить как то не так
type configurator struct {
	services       map[string]*serviceinstance
	servicesStatus map[string]int // TODO: write to memcacshed and add mutex
	hosts          map[string]struct{}
}

type serviceinstance struct {
	addr   string
	wsconn net.Conn
}

func (conf *config) readTomlConfig(path string) error {
	if _, err := toml.DecodeFile(path, conf); err != nil {
		return err
	}
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
				fmt.Println("Unknown servicename \"", servicename, "\"") //
				return ws.RejectConnectionError(
					ws.RejectionStatus(403),
				)
			}
			return nil
		},
		OnHost: func(host []byte) error {
			ind := bytes.Index(host, []byte{58}) // for cutting port
			if _, ok := c.hosts[string(host[:ind])]; !ok {
				fmt.Println("Unknown host \"", string(host), "\"")
				return ws.RejectConnectionError(
					ws.RejectionStatus(403),
				)
			}
			return nil
		},
		OnBeforeUpgrade: func() (header ws.HandshakeHeader, err error) {
			if c.services[servicename].addr == "*" {
				if c.services[servicename].addr, err = getfreeaddr(); err != nil {
					ws.RejectConnectionError(
						ws.RejectionStatus(500),
					)
				}
				c.servicesStatus[servicename] = 1
			}
			return ws.HandshakeHeaderHTTP(http.Header{
				"X-listen-here-u-little-shit": []string{c.services[servicename].addr},
			}), nil
		},
	}

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
	if err = poller.Start(desc, func(ev netpoll.Event) {
		if ev&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
			// When ReadHup or Hup received, this mean that client has
			// closed at least write end of the connection or connections
			// itself. So we want to stop receive events about such conn.
			poller.Stop(desc)
			conn.Close()
			fmt.Println("CLOSED", nameConn(conn), "at event", ev)
			return
		}

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

			fmt.Println("PAYLOAD:", string(payload)) //
		})
	}); err != nil {
		fmt.Println("Starting poller err:", err)
		conn.Close()
	}

}
func main() {

	conf := &config{}
	if err := conf.readTomlConfig("config.toml"); err != nil {
		fmt.Println("Read toml err:", err)
		return
	}

	ln, err := net.Listen("tcp", conf.Listen)
	if err != nil {
		fmt.Println("Listen err:", err)
		return
	}

	c := &configurator{services: make(map[string]*serviceinstance), servicesStatus: make(map[string]int), hosts: map[string]struct{}{}}
	if err := c.readsettings(conf.Settings); err != nil {
		fmt.Println("Reading settings:", err)
		return
	}
	c.hosts["127.0.0.1"] = struct{}{}

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

	if err = poller.Start(acceptDesc, func(e netpoll.Event) {
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

			log.Println("Accept err:", err)

		cooldown:
			delay := 5 * time.Millisecond
			log.Printf("Accept err: %v; retrying in %s", err, delay)
			time.Sleep(delay)
		}

		poller.Resume(acceptDesc)
	}); err != nil {
		fmt.Println("Start accept poller err:", err)
		return
	}
	<-exit
}

func (c *configurator) readsettings(settingspath string) error {
	data, err := ioutil.ReadFile(settingspath)
	if err != nil {
		return err
	}
	datastr := string(data)
	lines := strings.Split(datastr, "\n")
	for _, line := range lines {
		if len(line) < 2 || strings.HasPrefix(line, "#") {
			continue
		}
		s := strings.Split(strings.TrimSpace(line), " ")
		if len(s) < 2 {
			continue
		}
		c.services[s[0]] = &serviceinstance{addr: s[1]}
		c.servicesStatus[s[0]] = 0
	}
	return nil
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
