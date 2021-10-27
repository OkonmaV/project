package main

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Задача этой херни - писать логи в массив и скидывать кусками
type LoggerContainer struct {
	level LogLevel
	logs  []LogEntity
	mux   sync.Mutex
	flush chan []LogEntity
}

func NewLoggerContainer(ctx context.Context, minlevel LogLevel, capacity int, flushTimeout time.Duration) (*LoggerContainer, error) {
	if capacity <= 0 {
		return nil, errors.New("Logger capacity not set")
	}
	result := &LoggerContainer{
		level: minlevel,
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
				return
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

func (l *LoggerContainer) Write(time time.Time, level LogLevel, name, message string) {
	if level < l.level || name == "" || message == "" {
		return
	}

	l.mux.Lock()
	defer l.mux.Unlock()
	l.logs = append(l.logs, LogEntity{Time: time, Level: level, Name: name, Message: message})
	if len(l.logs) == cap(l.logs) {
		a := l.logs
		l.logs = make([]LogEntity, 0, cap(l.logs))
		l.flush <- a
	}
}

func (l *LoggerContainer) Flush() <-chan []LogEntity {
	return l.flush
}

func (l *LoggerContainer) Error(name string, err error) {
	l.Write(time.Now(), ErrorLevel, name, err.Error())
}
func (l *LoggerContainer) Warning(name, message string) {
	l.Write(time.Now(), WarningLevel, name, message)
}
func (l *LoggerContainer) Info(name, message string) {
	l.Write(time.Now(), InfoLevel, name, message)
}
func (l *LoggerContainer) Debug(name, message string) {
	l.Write(time.Now(), DebugLevel, name, message)
}
