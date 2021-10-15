package logscontainer

import (
	"errors"
	"time"
)

type WrappedLogsContainer struct {
	tags      LogTags
	container *LogsContainer
}

func (l *LogsContainer) Wrap(tags LogTags) *WrappedLogsContainer {
	if len(tags) == 0 || tags == nil {
		panic("tags must be non-nil and of length>0")
	}
	return &WrappedLogsContainer{tags: tags, container: l}
}

// return new wrappedlogs based on previous tags with adding new additional tags
func (wl *WrappedLogsContainer) ReWrap(additionalTags LogTags) *WrappedLogsContainer {
	if len(additionalTags) == 0 || additionalTags == nil {
		panic("additionaltags must be non-nil and of length>0")
	}
	for key, value := range wl.tags {
		if _, ok := additionalTags[key]; !ok {
			additionalTags[key] = value
		}
	}
	return &WrappedLogsContainer{tags: additionalTags, container: wl.container}
}

func (wl *WrappedLogsContainer) WaitAllFlushesDone() {
	wl.container.WaitAllFlushesDone()
}

func (wl *WrappedLogsContainer) SetTag(tag Tag, value string) {
	wl.tags.SetTag(tag, value)
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
	wl.container.logs = append(wl.container.logs, Log{Time: now, Description: descr, Lvl: lvl, Log: err, Tags: wl.tags.getcopy()})
}
