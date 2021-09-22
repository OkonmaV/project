package main

import (
	"context"
	"net"

	"project/test/auth/logscontainer"
	"project/test/auth/logscontainer/flushers"
	"thin-peak/httpservice"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/gobwas/ws"
)

type config struct {
	Listen   string
	Settings string
	//Hosts    []string
}

func main() {
	conf := &config{}
	if _, err := toml.DecodeFile("config.toml", conf); err != nil {
		println("Read toml err:", err)
		return
	}
	if conf.Listen == "" || conf.Settings == "" {
		println("Some fields in conf.toml are empty or not specified")
	}

	ctx, cancel := httpservice.CreateContextWithInterruptSignal()
	logsctx, logscancel := context.WithCancel(context.Background())

	l, err := logscontainer.NewLogsContainer(logsctx, flushers.NewConsoleFlusher("CNFG"), 1, time.Second, 1)
	if err != nil {
		println("Logs init err:", err)
		return
	}
	defer func() {
		cancel()
		logscancel()
		<-l.Done
	}()

	c, err := NewConfigurator(conf.Settings)
	if err != nil {
		l.Error("NewConfigurator", err)
		return
	}
	if err = c.Serve(ctx, l, conf.Listen); err != nil {
		l.Error("Serve", err)
	}

}

func SendTextToClient(conn net.Conn, text []byte) error {
	return ws.WriteFrame(conn, ws.NewTextFrame(text))
}

func SendTextToServer(conn net.Conn, text []byte) error {
	return ws.WriteFrame(conn, ws.MaskFrame(ws.NewTextFrame(text)))
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
