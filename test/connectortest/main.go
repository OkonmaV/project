package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"project/test/connector"
	"project/test/logs"
	"time"
)

func main() {

	ctx := context.Background()
	logcontainer, _ := logs.NewLoggerContainer(ctx, logs.DebugLevel, 10, time.Second*2)
	consolelogger := &logs.ConsoleLogger{}
	onlinelogger, _ := logs.NewOnlineLogger(logs.DebugLevel)
	go func() {
		for {
			select {
			case l := <-onlinelogger.Flush():
				logcontainer.Write(l.Time, l.Level, l.Name, l.Message)
			case l := <-logcontainer.Flush():
				consolelogger.WriteMany(l)
			}
		}
	}()

	// for i := 0; i < 9; i++ {
	// 	logcontainer.Debug("test", strconv.Itoa(i))
	// }

	listener, err := connector.NewListener("tcp", "127.0.0.1:9001")
	if err != nil {
		log.Fatalln(err)
	}
	conn, err := net.Dial("tcp", "127.0.0.1:9001")
	if err != nil {
		log.Fatalln(err)
	}
	connector, err := connector.NewConnector("mynameis", conn, func(message []byte) {
		onlinelogger.Debug("1", string(message))
	})
	if err != nil {
		log.Fatalln(err)
	}

	if err = connector.Send([]byte("mynameis")); err != nil {
		log.Fatalln(err)
	}

	time.Sleep(time.Millisecond * 50)

	if err = listener.Connections["mynameis"][0].Send([]byte("hello from server")); err != nil {
		log.Fatalln(err)
	}
	if err = connector.Send([]byte("hello from client")); err != nil {
		log.Fatalln(err)
	}
	onlinelogger.Debug("Done", "Done")
	fmt.Scanln()
	connector.Close()
	listener.Close()
	onlinelogger.Debug("Done", "Close")
	time.Sleep(time.Second)
}
