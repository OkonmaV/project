package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/big-larry/suckhttp"
)

func main1() {
	listener, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalln(err)
	}

	for {
		con, err := listener.Accept()
		if err != nil {
			log.Fatalln(err)
		}

		go handler(con)
	}
}

func handler(con net.Conn) {
	log.Println("Handling", con.RemoteAddr().String())
	for {
		req, err := suckhttp.ReadRequest(context.Background(), con, time.Minute)
		if err != nil {
			log.Fatalln(err)
		}
		con.SetDeadline(time.Time{})
		fmt.Println("Readed", con.RemoteAddr().String(), len(req.Bytes()))
		n, err := con.Write([]byte("HTTP/1.1 200 OK\r\nConnection: Keep-Alive\r\nContent-Length:0\r\n\r\n"))
		if err != nil {
			log.Fatalln(err)
		}
		con.SetDeadline(time.Time{})
		log.Println("Writed", n)
	}
}
