package main

import (
	"context"
	"fmt"
	"net/url"
	"project/test/httpservice"
	"project/test/messages/messagestypes"
	"project/test/repo/clickhouse"
	"project/test/types"
	"strings"

	"github.com/big-larry/suckhttp"
)

type config struct {
	ClickhouseAddr  []string
	ClickhouseTable string
}

type service struct {
	chconn *clickhouse.ClickhouseConnection
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

const thisServiceName httpservice.ServiceName = "messages.sendtext"

func (c *config) CreateHandler(ctx context.Context, pubs_getter httpservice.Publishers_getter) (httpservice.HTTPService, error) {
	conn, err := clickhouse.Connect(ctx, c.ClickhouseAddr, c.ClickhouseTable, "default", "", "", 0, 0)
	if err != nil {
		return nil, err
	}
	s := &service{chconn: conn}
	if err := s.chconn.Conn.Exec(ctx, crt); err != nil {
		panic(err)
	}
	return s, nil
}

func (s *service) Handle(r *suckhttp.Request, l types.Logger) (*suckhttp.Response, error) {
	if r.GetMethod() != suckhttp.POST || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "application/x-www-form-urlencoded") || len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	chatId := strings.Trim(r.Uri.Path, "/")

	// SKIPPING COOKIE CHECK
	userId := "testuserid" // SKIPPING SKIPPED COOKIE DECODING
	// SKIPPING CHATID CHECK AND USER'S RIGHTS CHECK

	fvalues, err := url.ParseQuery(string(r.Body))
	if err != nil {
		l.Debug("ParseQuery", err.Error())
		return suckhttp.NewResponse(400, "Bad request"), nil
	}
	message := fvalues.Get("message")
	if len(message) == 0 {
		l.Debug("Request", "empty message")
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	if err := s.chconn.Insert(fmt.Sprintf("'%s','%s',%s,'%s',%s", userId, chatId, messagestypes.Text.String(), message, "now()")); err != nil {
		return suckhttp.NewResponse(500, "Internal Server Error"), err
	}

	return suckhttp.NewResponse(200, "OK"), nil
}

func (s *service) Close() error {
	return s.chconn.Close()
}

func main() {
	httpservice.InitNewServiceWithoutConfigurator(thisServiceName, &config{}, false, 1)
}
