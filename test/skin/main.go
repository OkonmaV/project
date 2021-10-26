package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"project/test/logscontainer"
	"project/test/logscontainer/flushers"
	"strconv"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/big-larry/suckutils"
)

type config struct {
	Configurator string
}

type testconn struct {
	status bool
	name   string
}

type toConfDataStream chan []string
type toServDataStream chan map[string][]string

const thissetvicename string = "skin"

func dial(l *logscontainer.LogsContainer, conn *testconn) (toConfDataStream, toServDataStream) {
	rand.Seed(time.Now().UnixNano())
	conn.status = true
	conn.name = "SERV-CONF-CONN-" + strconv.Itoa(rand.Intn(100))
	toconf := make(toConfDataStream, 1)
	toserv := make(toServDataStream, 1)

	return toconf, toserv
}

func testconfigurator(conn *testconn, toconf toConfDataStream, toserv toServDataStream, l logscontainer.Logger) {
	db := map[string][]string{"first": {"11", "12"}, "second": {"21", "22"}}
	go func() {
		time.Sleep(time.Duration(time.Second * 4))
		toserv <- map[string][]string{"first": {"121"}}
		l.Info("Servconf", "{\"first\":  \"121\"}} update sended")
		time.Sleep(time.Duration(time.Second * 4))
		toserv <- map[string][]string{"second": {}}
		l.Info("Servconf", "{\"second\": {}} update sended")
	}()
	for {
		if conn.status {
			select {
			case msg := <-toconf:
				if len(msg) > 0 {
					resp := make(map[string][]string, len(msg))
					for _, sn := range msg {
						resp[sn] = db[sn]
					}
					toserv <- resp
				}
			default:
				continue
			}
		} else {
			l.Info("Serv", "off")
			l.Info("Conn", "conn is down, coldowning for 3s")
			time.Sleep(time.Duration(time.Second * 3))
		}
	}
}

type pubsdata struct {
	pubs map[string][]string
	rw   sync.RWMutex
}

func (d *pubsdata) handle(l logscontainer.Logger, ctx context.Context, pubs toServDataStream) {
	for {
		select {
		case <-ctx.Done():
			l.Info("handle", "context done, exiting function")
			return
		case pubsupdate := <-pubs:
			d.rw.Lock()
			for k, v := range pubsupdate {
				if _, ok := d.pubs[k]; ok {
					d.pubs[k] = v
					l.Debug("handle", suckutils.ConcatTwo("updated pub: ", k))
				} else {
					l.Warning("handle", suckutils.ConcatTwo("unknown pub at pubsupdate: ", k))
				}
			}
			d.rw.Unlock()
		}
	}
}

func main() {
	ctx, _, confAddr, l, waitlogs, err := newService(thissetvicename)
	if err != nil {
		println(err.Error())
		return
	}
	defer waitlogs()

	conn := &testconn{}
	toconf, toserv := dial(l, conn)
	toconf <- []string{"first", "second"}
	go testconfigurator(conn, toconf, toserv, l.Wrap(map[logscontainer.Tag]string{logscontainer.TagTest: confAddr}))

	d := &pubsdata{pubs: map[string][]string{"first": nil, "second": nil}}
	go func() {
		for {
			d.rw.RLock()
			l.Info("PUBS", mapstringer(d.pubs))
			d.rw.RUnlock()
			time.Sleep(time.Second)
		}
	}()

	d.handle(l, ctx, toserv)
}

func mapstringer(m map[string][]string) string {
	b := new(bytes.Buffer)
	for key, value := range m {
		fmt.Fprintf(b, "%s=\"%s\"\n", key, value)
	}
	return b.String()
}

func newService(servname string) (context.Context, context.CancelFunc, string, *logscontainer.LogsContainer, func(), error) {
	conf := &config{}
	if _, err := toml.DecodeFile("config.toml", conf); err != nil {
		return nil, nil, "", nil, nil, err
	}
	if len(conf.Configurator) == 0 {
		return nil, nil, "", nil, nil, errors.New("some fields in conf.toml are empty or not specified")
	}
	ctx, cancel := CreateContextWithInterruptSignal()
	logsctx, logscancel := context.WithCancel(context.Background())

	l, err := logscontainer.NewLogsContainer(logsctx, flushers.NewConsoleFlusher(servname), 1, time.Second, 1)
	if err != nil {
		return nil, nil, "", nil, nil, err
	}

	return ctx, cancel, conf.Configurator, l, func() {
		logscancel()
		l.WaitAllFlushesDone()
	}, nil
}

func CreateContextWithInterruptSignal() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		<-stop
		cancel()
	}()
	return ctx, cancel
}
