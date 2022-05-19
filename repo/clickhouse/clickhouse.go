package repoClickhouse

import (
	"context"
	"errors"

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
	clkhs.insert_query_prefix = suckutils.ConcatThree("INSERT INTO ", tablename, " ")
	return clkhs, nil
}

func (clkhs *ClickhouseConnection) Close() error {
	if clkhs.Conn != nil {
		return clkhs.Conn.Close()
	}
	return nil
}

func (clkhs *ClickhouseConnection) Insert(value string) error {
	if len(value) == 0 {
		return errors.New("no value to write")
	}
	return clkhs.Conn.AsyncInsert(clkhs.ctx, suckutils.ConcatTwo(clkhs.insert_query_prefix, value), true)
}

func (clkhs *ClickhouseConnection) InsertJSON(s string) error {
	if len(s) == 0 {
		return errors.New("no value to write")
	}
	return clkhs.Conn.AsyncInsert(clkhs.ctx, suckutils.Concat(clkhs.insert_query_prefix, " format JSONEachRow ", s), true)
}

// TODO: если не придумаю как экранировать в строке в запросе спецсимволы типа '' (для AsyncInsert()), то ебнуть горутину на инсерт
// func (clkhs *ClickhouseConnection) InsertBatch(v interface{}) error {
// 	if v == nil {
// 		return errors.New("nil value")
// 	}
// 	batch, err := clkhs.Conn.PrepareBatch(clkhs.ctx, clkhs.bquery)
// 	if err != nil {
// 		return err
// 	}
// 	if err := batch.AppendStruct(v); err != nil {
// 		return err
// 	}

// 	return batch.Send()

// }
