package logscontainer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

type LogsContainer struct {
	logs        []Log
	addlogmutex sync.Mutex
	flushing    chan []Log
	// Сигнал при завершении работы, сообщающий что новые флуши инициализироваться не будут, а значит все новые добавляемые логи не обрабатываются (выкидываются)
	doneflushing   bool
	allflushesdone chan struct{}
}

type Log struct {
	Time        time.Time
	Description string
	Lvl         loglevel
	Log         error
	Tags        []string
}

type LogsFlusher interface {
	Flush([]Log) error
}
type loglevel uint8

const (
	debug   = 1
	info    = 2
	warning = 3
	err     = 4
)

var defaultLastFlushesTimeout time.Duration = time.Second * 20

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

func NewLogsContainer(ctx context.Context, f LogsFlusher, capacity int, flushperiod time.Duration, minflushinglen int) (*LogsContainer, error) {

	if capacity < 1 {
		return nil, errors.New("capacity < 1")
	}
	if minflushinglen < 1 {
		return nil, errors.New("minflushinglen < 1")
	}
	if minflushinglen > capacity {
		return nil, errors.New("minflushinglen > capacity")
	}

	l := &LogsContainer{
		logs:           make([]Log, 0, capacity),
		addlogmutex:    sync.Mutex{},
		flushing:       make(chan []Log, 1),
		allflushesdone: make(chan struct{}, 1),
	}

	go listener(ctx, f, l, flushperiod, minflushinglen)

	return l, nil
}

var deflogs *LogsContainer

func SetupDefault() {
	deflogs = &LogsContainer{
		logs:           make([]Log, 0, 2),
		addlogmutex:    sync.Mutex{},
		flushing:       make(chan []Log, 1),
		allflushesdone: make(chan struct{}, 1),
	}

	go listener(context.Background(), &defflusher{name: "main"}, deflogs, time.Second*1, 1)
}

func (l *LogsContainer) WaitAllFlushesDone() {
	timer := time.NewTimer(defaultLastFlushesTimeout)
	select {
	case <-l.allflushesdone:
		return
	case <-timer.C:
		println("[", time.Now().UTC().String(), "] Last flushes interrupted because of timeout ", defaultLastFlushesTimeout.String())
		return
	}
}
func Error(descr string, data error) {
	deflogs.addlog(descr, err, data)
}

func Debug(descr string, data string) {
	deflogs.addlog(descr, debug, errors.New(data))
}
func Warning(descr string, data string) {
	deflogs.addlog(descr, warning, errors.New(data))
}
func Info(descr string, data string) {
	deflogs.addlog(descr, info, errors.New(data))
}

func (l *LogsContainer) Error(descr string, data error) {
	l.addlog(descr, err, data)
}
func (l *LogsContainer) Debug(descr string, data string) {
	l.addlog(descr, debug, errors.New(data))
}
func (l *LogsContainer) Warning(descr string, data string) {
	l.addlog(descr, warning, errors.New(data))
}
func (l *LogsContainer) Info(descr string, data string) {
	l.addlog(descr, info, errors.New(data))
}

func (l *LogsContainer) addlog(descr string, lvl loglevel, err error) {
	now := time.Now()
	l.addlogmutex.Lock()
	defer l.addlogmutex.Unlock()

	if l.doneflushing || err == nil {
		return
	}
	if len(l.logs) == cap(l.logs) {
		l.flushing <- l.logs
		l.logs = make([]Log, 0, cap(l.logs))
	}
	l.logs = append(l.logs, Log{Time: now, Description: descr, Lvl: lvl, Log: err})
}

func listener(ctx context.Context, f LogsFlusher, l *LogsContainer, flushperiod time.Duration, minflushinglen int) {

	ticker := time.NewTicker(flushperiod)
	go func() {
		for errpack := range l.flushing {
			if err := f.Flush(errpack); err != nil {
				//errs.AddError(err)
			}
		}
		l.allflushesdone <- struct{}{}
	}()
	go func() {
		for range ticker.C {
			l.addlogmutex.Lock()

			if len(l.logs) >= minflushinglen {
				l.flushing <- l.logs
				l.logs = make([]Log, 0, cap(l.logs))
			}
			l.addlogmutex.Unlock()
		}
	}()

	<-ctx.Done()

	l.addlogmutex.Lock()
	ticker.Stop()

	l.doneflushing = true
	l.flushing <- l.logs
	close(l.flushing)
	//<-errs.done
	l.addlogmutex.Unlock()
}

type defflusher struct {
	name string
}

func (c *defflusher) Flush(logs []Log) error {
	for i := 0; i < len(logs); i++ {
		fmt.Fprintf(os.Stdout, "[%s] [%s] [%s] [%s] %s\n", logs[i].Lvl, logs[i].Description, c.name, logs[i].Time.Format("2006-01-02T15:04:05Z07:00"), logs[i].Log.Error())
	}
	return nil
}
