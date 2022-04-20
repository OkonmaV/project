package main

import (
	"context"
	"errors"
	"net"

	"project/test/repo/clickhouse"
	"project/test/types"
	"project/test/wsconnector"
	"project/test/wsservice"
	"strings"

	"github.com/big-larry/suckhttp"
	"github.com/gobwas/ws"
	"github.com/gorilla/websocket"
)

type config struct {
	ClickhouseAddr  []string
	ClickhouseTable string
}

type service struct {
	chconn *clickhouse.ClickhouseConnection
	*ws.Upgrader

	path string
}

const crt = `
CREATE TABLE IF NOT EXISTS messagestest (
	  UserId String
	, ChatId String
	, MessageType UInt8
	, Message String
	, Time DateTime
) ENGINE = MergeTree()
ORDER BY Time
`

const thisServiceName wsservice.ServiceName = "messages.get"

type userid string

type hub struct {
	subscriptors map[userid][]net.Conn
}

type subwsconns struct {
}

type sub struct {
	userId userid
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// один вебсокет чтобы править всеми // 12byte objectid

func (c *config) CreateHandler(ctx context.Context, pubs_getter wsservice.Publishers_getter) (wsservice.HTTPService, ws.Upgrader, error) {
	conn, err := clickhouse.Connect(ctx, c.ClickhouseAddr, c.ClickhouseTable, "default", "", "", 0, 0)
	if err != nil {
		return nil, err
	}
	if err := conn.Conn.Exec(ctx, crt); err != nil {
		panic(err)
	}

	return &service{chconn: conn}, nil
}

func (s *service) Handle(r *suckhttp.Request, l types.Logger) (*suckhttp.Response, error) {
	if r.GetMethod() != suckhttp.GET {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	chatId := strings.Trim(r.Uri.Path, "/")

	// SKIPPING COOKIE CHECK
	userId := "testuserid" // SKIPPING SKIPPED COOKIE DECODING
	// SKIPPING CHATID CHECK AND USER'S RIGHTS CHECK
	conn, _ := upgrader.Upgrade(nil, nil, nil)
	websocket.Upgrade()
	return suckhttp.NewResponse(200, "OK"), nil
}

func (s *service) Close() error {
	return s.chconn.Close()
}

func main() {
	wsservice.InitNewServiceWithoutConfigurator(thisServiceName, &config{}, false, 1)
}

func (s *service) HandleWSInit(conn net.Conn, l types.Logger) error {
	wsconnector.SetupConnUpgrader()
	chatId := strings.Trim(r.Uri.Path, "/")
	if len(chatId) == 0 {
		return errors.New("empty path = no chatId")
	}
	// SKIPPING COOKIE CHECK
	userId := "testuserid" // SKIPPING SKIPPED COOKIE DECODING
	// SKIPPING CHATID CHECK AND USER'S RIGHTS CHECK
	wsconnector.NewWSConnector(conn)
	if _, err := ws.Upgrade(conn); err != nil {
		return err
	}

}
