package logscontainer

import (
	"bytes"
	"fmt"
)

type LogTags map[string]string

func (tags LogTags) String() string {
	b := new(bytes.Buffer)
	for key, value := range tags {
		fmt.Fprintf(b, "[%s:%s]", key, value)
	}
	return b.String()
}

func (tags LogTags) AddTag(key, value string) {
	tags[key] = value
}

func (tags LogTags) Reset() {
	tags = make(map[string]string)
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
