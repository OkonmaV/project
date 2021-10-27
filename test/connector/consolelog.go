package main

import (
	"strings"
	"time"

	"github.com/big-larry/suckutils"
)

type ConsoleLogger struct {
}

func (l *ConsoleLogger) Write(time time.Time, level LogLevel, name, message string) {
	println(time.Format("02/01/2006 03:04:05.000000"), level.String(), name, message)
}
func (l *ConsoleLogger) WriteMany(logs []LogEntity) {
	buf := strings.Builder{}
	for _, l := range logs {
		buf.WriteString(suckutils.Concat(l.Time.Format("02/01/2006 03:04:05.000000"), " ", l.Level.String(), " ", l.Name, " ", l.Message, "\r\n"))
	}
	println(buf.String())
}

func (l *ConsoleLogger) Error(name string, err error) {
	l.Write(time.Now(), ErrorLevel, name, err.Error())
}
func (l *ConsoleLogger) Warning(name, message string) {
	l.Write(time.Now(), WarningLevel, name, message)
}
func (l *ConsoleLogger) Info(name, message string) {
	l.Write(time.Now(), InfoLevel, name, message)
}
func (l *ConsoleLogger) Debug(name, message string) {
	l.Write(time.Now(), DebugLevel, name, message)
}
