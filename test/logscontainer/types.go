package logscontainer

import (
	"bytes"
	"fmt"
)

type LogTags map[Tag]string

type Tag uint8

const (
	TagNameOfConnectedService       Tag = 1
	TagRemoteAddr                   Tag = 2
	TagRequestId                    Tag = 3
	TagListenAddrOfConnectedService Tag = 4
)

func (tag Tag) String() string {
	switch tag {
	case TagNameOfConnectedService:
		return "name-of-connected-service"
	case TagRemoteAddr:
		return "remote-addr"
	case TagRequestId:
		return "req-id"
	case TagListenAddrOfConnectedService:
		return "connected-serv-ln-at"
	default:
		return "undefined-tag"
	}
}

func (tags LogTags) String() string {
	b := new(bytes.Buffer)
	for key, value := range tags {
		fmt.Fprintf(b, "[%s:%s]", key.String(), value)
	}
	return b.String()
}

func (tags LogTags) SetTag(key Tag, value string) {
	tags[key] = value
}

func (tags LogTags) Reset() {
	tags = make(map[Tag]string)
}

func (tags LogTags) getcopy() LogTags {
	copied := make(LogTags, len(tags))
	for k, v := range tags {
		copied[k] = v
	}
	return copied
}

type loglevel uint8

func (l loglevel) String() string {
	switch l {
	case debug:
		return "DBG"
	case info:
		return "INF"
	case warning:
		return "WRN"
	case err:
		return "ERR"
	}
	return "UNK"
}

type LogsFlusher interface {
	Flush([]Log) error
}

type Logger interface {
	Error(string, error)
	Debug(string, string)
	Warning(string, string)
	Info(string, string)
}
