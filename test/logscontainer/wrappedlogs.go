package logscontainer

import (
	"errors"
	"time"
)

type WrappedLogsContainer struct {
	// [remoteaddr,requestid]
	tags      []string
	container *LogsContainer
}

func (l *LogsContainer) Wrap(tags ...string) *WrappedLogsContainer {
	if len(tags) == 0 {
		panic("unnecessary use of logscontainer wrapper")
	}

	return &WrappedLogsContainer{tags: tags, container: l}
}

func (wl *WrappedLogsContainer) WaitAllFlushesDone() {
	wl.container.WaitAllFlushesDone()
}

func (wl *WrappedLogsContainer) Error(descr string, data error) {
	wl.addlog(descr, err, data)
}
func (wl *WrappedLogsContainer) Debug(descr string, data string) {
	wl.addlog(descr, debug, errors.New(data))
}
func (wl *WrappedLogsContainer) Warning(descr string, data string) {
	wl.addlog(descr, warning, errors.New(data))
}
func (wl *WrappedLogsContainer) Info(descr string, data string) {
	wl.addlog(descr, info, errors.New(data))
}

func (wl *WrappedLogsContainer) addlog(descr string, lvl loglevel, err error) {
	now := time.Now()
	wl.container.addlogmutex.Lock()
	defer wl.container.addlogmutex.Unlock()

	if wl.container.doneflushing || err == nil {
		return
	}
	if len(wl.container.logs) == cap(wl.container.logs) {
		wl.container.flushing <- wl.container.logs
		wl.container.logs = make([]Log, 0, cap(wl.container.logs))
	}
	wl.container.logs = append(wl.container.logs, Log{Time: now, Description: descr, Lvl: lvl, Log: err, Tags: wl.tags})
}
