package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"project/test/httpservice"
	"project/test/messages/messagestypes"
	"project/test/repo/clickhouse"
	"project/test/types"
	"strconv"
	"strings"
	"time"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type config struct {
	ClickhouseAddr  []string
	ClickhouseTable string
	FilesPath       string
}

type service struct {
	chconn *clickhouse.ClickhouseConnection
	path   string
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

const thisServiceName httpservice.ServiceName = "messages.sendfile"

func (c *config) CreateHandler(ctx context.Context, pubs_getter httpservice.Publishers_getter) (httpservice.HTTPService, error) {
	if len(c.FilesPath) == 0 {
		return nil, errors.New("FilesPath not set")
	}
	if stat, err := os.Stat(c.FilesPath); err != nil {
		return nil, err
	} else if !stat.IsDir() {
		return nil, errors.New("FilePath is not a directory")
	}

	conn, err := clickhouse.Connect(ctx, c.ClickhouseAddr, c.ClickhouseTable, "default", "", "", 0, 0)
	if err != nil {
		return nil, err
	}
	if err := conn.Conn.Exec(ctx, crt); err != nil {
		panic(err)
	}
	return &service{chconn: conn, path: c.FilesPath}, nil
}

func (s *service) Handle(r *suckhttp.Request, l types.Logger) (*suckhttp.Response, error) {
	if r.GetMethod() != suckhttp.POST || !strings.Contains(r.GetHeader(suckhttp.Content_Type), "multipart/form-data") || len(r.Body) == 0 {
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	chatId := strings.Trim(r.Uri.Path, "/")

	// SKIPPING COOKIE CHECK
	userId := "testuserid" // SKIPPING SKIPPED COOKIE DECODING
	// SKIPPING CHATID CHECK AND USER'S RIGHTS CHECK

	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return suckhttp.NewResponse(500, "Internal Server Error"), err
	}

	chat_dir := suckutils.Concat(s.path, "/", chatId)
	if _, err := os.Stat(chat_dir); os.IsNotExist(err) {
		if err = os.Mkdir(chat_dir, 0755); err != nil {
			return suckhttp.NewResponse(500, "Internal Server Error"), err
		}
	}
	mt, params, err := mime.ParseMediaType(r.GetHeader(suckhttp.Content_Type))
	if err != nil {
		return suckhttp.NewResponse(400, "Bad Request"), err
	}
	messagetype := messagestypes.Parse(mt)
	if messagetype == messagestypes.Unknown {
		l.Warning("Mediatype", suckutils.ConcatTwo("is unknown: ", mt))
		return suckhttp.NewResponse(400, "Bad Request"), nil
	}

	var filename, filepath string
	var filedata []byte
	mpreader := multipart.NewReader(bytes.NewReader(r.Body), params["boundary"])
	for {
		part, err := mpreader.NextPart()
		if err == io.EOF {
			break
		}
		l.Debug("Upload", part.FileName())
		if err != nil {
			return suckhttp.NewResponse(400, "Bad request"), err
		}
		piece, err := io.ReadAll(part)
		if err != nil {
			return suckhttp.NewResponse(400, "Bad request"), err
		}
		if part.FormName() == "file" {
			filedata = piece
			filename = part.FileName()

			if filename == "" {
				l.Debug("Filename", "is empty")
				return suckhttp.NewResponse(400, "Bad request"), nil
			}
			filepath = suckutils.Concat(chat_dir, "/", strconv.FormatInt(time.Now().UnixMicro(), 10), "/-/", userId, "/-/", filename)
			if err = os.WriteFile(filepath, filedata, 0644); err != nil {
				return suckhttp.NewResponse(500, "Internal Server Error"), err
			}
			if err := s.chconn.Insert(fmt.Sprintf("'%s','%s',%s,'%s',%s", userId, chatId, messagetype.String(), filepath, "now()")); err != nil {
				if err = os.Remove(filepath); err != nil {
					l.Error("Remove", err)
				}
				return suckhttp.NewResponse(500, "Internal Server Error"), err
			}
			filename = ""
		}
	}

	return suckhttp.NewResponse(200, "OK"), nil
}

func (s *service) Close() error {
	return s.chconn.Close()
}

func main() {
	httpservice.InitNewServiceWithoutConfigurator(thisServiceName, &config{}, false, 1)
}
