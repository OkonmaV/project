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
	logs         []Log
	addlogmutex  sync.Mutex
	flushing     chan []Log
	doneflushing bool
	Done         chan struct{}
}

type Log struct {
	Time        time.Time
	Description string
	Type        string
	Log         error
}

type LogsFlusher interface {
	Flush([]Log) error
}

const (
	debug   string = "DBG"
	info    string = "INF"
	err     string = "ERR"
	warning string = "WRN"
)

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
		logs:        make([]Log, 0, capacity),
		addlogmutex: sync.Mutex{},
		flushing:    make(chan []Log, 1),
		Done:        make(chan struct{}, 1),
	}

	go listener(ctx, f, l, flushperiod, minflushinglen)

	return l, nil
}

var deflogs *LogsContainer

func Setup() {
	deflogs = &LogsContainer{
		logs:        make([]Log, 0, 10),
		addlogmutex: sync.Mutex{},
		flushing:    make(chan []Log, 1),
		Done:        make(chan struct{}, 1),
	}

	go listener(context.Background(), &defflusher{name: "main"}, deflogs, time.Second*2, 1)
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

func (l *LogsContainer) addlog(descr string, typee string, err error) {
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
	l.logs = append(l.logs, Log{Time: now, Description: descr, Type: typee, Log: err})
}

func listener(ctx context.Context, f LogsFlusher, l *LogsContainer, flushperiod time.Duration, minflushinglen int) {

	ticker := time.NewTicker(flushperiod)
	go func() {
		for errpack := range l.flushing {
			if err := f.Flush(errpack); err != nil {
				//errs.AddError(err)
			}
		}
		l.Done <- struct{}{}
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
		fmt.Fprintf(os.Stdout, "[%s] [%s] [%s] [%s] %s\n", logs[i].Type, logs[i].Description, c.name, logs[i].Time.Format("2006-01-02T15:04:05Z07:00"), logs[i].Log.Error())
	}
	return nil
}
