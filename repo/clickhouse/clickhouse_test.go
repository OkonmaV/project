package repoClickhouse_test

import (
	"context"
	repo_clickhouse "project/repo/clickhouse"
	"strings"
	"testing"
	"time"

	"github.com/big-larry/suckutils"
)

func TestAsyncInsert(t *testing.B) {
	start := make(chan struct{})
	end := make(chan struct{}, 2)
	end <- struct{}{}
	end <- struct{}{}
	pref := "INSERT INTO messagestest VALUES ("
	values := []string{"bench", "bench", "0", "bench", "now()"}

	conn, _ := connect()
	for i := 0; i < 3; i++ {
		go func() {
			<-start
			for i := 0; i < 1000; i++ {
				conn.Conn.AsyncInsert(context.Background(), suckutils.ConcatThree(pref, strings.Join(values, ", "), ")"), true)
			}
			<-end
		}()
	}
	time.Sleep(time.Second)
	t.ResetTimer()
	close(start)
	end <- struct{}{}
	end <- struct{}{}
}

func connect() (*repo_clickhouse.ClickhouseConnection, error) {
	return repo_clickhouse.Connect(context.Background(), []string{"127.0.0.1:9000"}, "messagestest", "default", "", "", 0, 0)
}

func worker(call func(), wait chan struct{}, iter int) {
	<-wait
	for i := 0; i < iter; i++ {
		call()
	}
}
