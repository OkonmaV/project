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
	Listen    string
	Settings  string
	TrntlAddr string
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

	c, err := NewConfigurator(conf.Settings, conf.TrntlAddr)
	if err != nil {
		l.Error("NewConfigurator", err)
		return
	}
	defer func() {
		if err = c.CloseTarantoolWithUpdateStatus(l); err != nil {
			l.Error("CloseTrntlWithUpdateStatus", err)
		}
	}()
	if err = c.Serve(ctx, l, conf.Listen); err != nil {
		l.Error("Serve", err)
	}

}

func SendTextToClient(conn net.Conn, text []byte) error {
	return ws.WriteFrame(conn, ws.NewTextFrame(text))
}
