package logs

import (
	"time"
)

// Эта херня сразу сбрасывает, но не блочит вызов
type OnlineLogger struct {
	level LogLevel
	logs  chan LogEntity
}

func NewOnlineLogger(minlevel LogLevel) (*OnlineLogger, error) {
	return &OnlineLogger{level: minlevel, logs: make(chan LogEntity, 10)}, nil
}

func (l *OnlineLogger) Flush() <-chan LogEntity {
	return l.logs
}

func (l *OnlineLogger) Write(time time.Time, level LogLevel, name, message string) {
	if level < l.level || name == "" || message == "" {
		return
	}
	l.logs <- LogEntity{Time: time, Level: level, Name: name, Message: message}
}

func (l *OnlineLogger) Error(name string, err error) {
	l.Write(time.Now(), ErrorLevel, name, err.Error())
}
func (l *OnlineLogger) Warning(name, message string) {
	l.Write(time.Now(), WarningLevel, name, message)
}
func (l *OnlineLogger) Info(name, message string) {
	l.Write(time.Now(), InfoLevel, name, message)
}
func (l *OnlineLogger) Debug(name, message string) {
	l.Write(time.Now(), DebugLevel, name, message)
}
