package main

import (
	"context"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
	"github.com/roistat/go-clickhouse"
)

type config struct {
	Configurator    string
	Listen          string
	MgoAddr         string
	MgoDB           string
	MgoColl         string
	ClickhouseAddr  string
	ClickhouseTable string
	mgoSession      *mgo.Session
}

var thisServiceName httpservice.ServiceName = "messages.sendtextmessage"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	mgoSession, err := mgo.Dial(c.MgoAddr)
	if err != nil {
		return nil, err
	}
	logger.Info("Mongo", "Connected!")
	mgoCollection := mgoSession.DB(c.MgoDB).C(c.MgoColl)

	chConn := clickhouse.NewConn(c.ClickhouseAddr, clickhouse.NewHttpTransport())
	//"CREATE TABLE IF NOT EXISTS main.chats (time DateTime,chatID UUID,user String,text String) ENGINE = MergeTree() ORDER BY tuple()"
	err = chConn.Ping()
	if err != nil {
		return nil, err
	}
	logger.Info("Clickhouse", "Connected!")
	return NewHandler(mgoCollection, chConn, c.ClickhouseTable)
}

func (conf *config) Close() error {
	conf.mgoSession.Close()
	return nil
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 5, &config{})
}
