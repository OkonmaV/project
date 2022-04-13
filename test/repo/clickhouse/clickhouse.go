package clickhouse

import (
	"context"
	"errors"
	"strings"

	"github.com/ClickHouse/clickhouse-go"
	"github.com/big-larry/suckutils"
)

type ClickhouseConnection struct {
	Conn                clickhouse.Conn
	insert_query_prefix string

	ctx context.Context
}

// todo: настройка wait_for_async_insert_timeout на сервере установлена в дефолт lock_acquire_timeout (у которого дефолт 120сек),
// т.е. если не смог воткнуть мьютекс за это время - вернет эксепшн (т.е. данные не вставятся). вопрос - че будет с данными, которые ждали этой вставки

// maxopenconns, maxidleconns = 10, 5 by default
func Connect(ctx context.Context, addr []string, tablename, dbname, username, password string, maxopenconns, maxidleconns int) (*ClickhouseConnection, error) {
	clkhs := &ClickhouseConnection{}
	var err error
	if clkhs.Conn, err = clickhouse.Open(&clickhouse.Options{
		Addr: addr,
		Auth: clickhouse.Auth{
			Database: dbname,
			Username: username,
			Password: password,
		},
		MaxOpenConns: maxopenconns,
		MaxIdleConns: maxidleconns,
	}); err != nil {
		return nil, err
	}

	if err = clkhs.Conn.Ping(ctx); err != nil {
		return nil, errors.New("clickhouse ping err:" + err.Error())
	}
	clkhs.ctx = ctx
	clkhs.insert_query_prefix = suckutils.ConcatThree("INSERT INTO ", tablename, " VALUES (")
	return clkhs, nil
}

func (clkhs *ClickhouseConnection) Close() error {
	if clkhs.Conn != nil {
		return clkhs.Conn.Close()
	}
	return nil
}

func (clkhs *ClickhouseConnection) Insert(values ...string) error {
	if len(values) == 0 {
		return errors.New("no values to write")
	}
	println("QUERY:" + suckutils.ConcatThree(clkhs.insert_query_prefix, strings.Join(values, ","), ")"))
	return clkhs.Conn.AsyncInsert(clkhs.ctx, suckutils.ConcatThree(clkhs.insert_query_prefix, strings.Join(values, ", "), ")"), true)
}
