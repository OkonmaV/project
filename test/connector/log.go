package main

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Logger interface {
	Error(string, error)
	Debug(string, string)
	Warning(string, string)
	Info(string, string)
}

// Задача этой херни - писать логи в массив и заполнять канал флушами
type LoggerContainer struct {
	name  string
	logs  []LogEntity
	mux   sync.Mutex
	flush chan []LogEntity
}

type ConsoleLogger struct {
}

type LogLevel uint8

type LogEntity struct {
	Time    time.Time
	Name    string
	Message string
	Level   LogLevel
}

const (
	debug LogLevel = iota + 1
	info
	warning
	err
)

func (l LogLevel) String() string {
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

func NewLogger(ctx context.Context, name string, capacity int, flushTimeout time.Duration) (*LoggerContainer, error) {
	if name == "" {
		return nil, errors.New("Logger name not set")
	}
	if capacity <= 0 {
		return nil, errors.New("Logger capacity not set")
	}
	result := &LoggerContainer{
		name:  name,
		logs:  make([]LogEntity, 0, capacity),
		flush: make(chan []LogEntity, 5),
		mux:   sync.Mutex{},
	}
	go func() {
		ticker := time.NewTicker(flushTimeout)
		for {
			select {
			case <-ctx.Done():
				result.doPartialFlush()
				break
			case <-ticker.C:
				result.doPartialFlush()
				break
			}
		}
	}()
	return result, nil
}

func (l *LoggerContainer) doPartialFlush() {
	l.mux.Lock()
	if len(l.logs) > 0 {
		a := l.logs
		l.logs = make([]LogEntity, 0, cap(l.logs))
		l.flush <- a
	}
	l.mux.Unlock()
}

func (l *LoggerContainer) Write(level LogLevel, name, message string) {
	if name == "" || message == "" {
		return
	}

	l.mux.Lock()
	defer l.mux.Unlock()
	l.logs = append(l.logs, LogEntity{Time: time.Now(), Level: level, Name: name, Message: message})
	if len(l.logs) == cap(l.logs) {
		a := l.logs
		l.logs = make([]LogEntity, 0, cap(l.logs))
		l.flush <- a
	}
}

func (l *LoggerContainer) Flush() <-chan []LogEntity {
	return l.flush
}
