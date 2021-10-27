package main

import (
	"time"
)

type Logger interface {
	Error(string, error)
	Debug(string, string)
	Warning(string, string)
	Info(string, string)
	Write(time time.Time, level LogLevel, name, message string)
	WriteMany([]LogEntity)
}

type LogLevel uint8

type LogEntity struct {
	Time    time.Time
	Name    string
	Message string
	Level   LogLevel
}

const (
	DebugLevel LogLevel = iota + 1
	InfoLevel
	WarningLevel
	ErrorLevel
)

func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DBG"
	case InfoLevel:
		return "INF"
	case WarningLevel:
		return "WRN"
	case ErrorLevel:
		return "ERR"
	}
	return "UNK"
}
