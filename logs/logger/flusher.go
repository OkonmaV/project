package logger

import (
	"project/logs/encode"
	"time"
)

type Flusher struct {
	ch       chan [][]byte
	flushlvl encode.LogsFlushLevel
	done     chan struct{}
}

var flushertags []byte

// when nonlocal = true, flush logs to logsservers, when logsserver is not available, saves logs for further flush to this server on reconnect
func NewFlusher(nonlocal bool) {

}

func (f *Flusher) Done() {
	<-f.done
}
func (f *Flusher) DoneWithTimeout(timeout time.Duration) {
	t := time.NewTimer(timeout)
	select {
	case <-f.done:
		return
	case <-t.C:
		encode.PrintLog(encode.EncodeLog(encode.Error, flushertags, "DoneWithTimeout", "reached timeout, skip last flush"))
		return
	}

}
