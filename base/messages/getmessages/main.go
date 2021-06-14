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
	MgoDB           string
	MgoAddr         string
	MgoColl         string
	ClickhouseAddr  string
	ClickhouseTable string
	mgoSession      *mgo.Session
}

const thisServiceName httpservice.ServiceName = "messages.getmessages"
const tokenDecoderServiceName httpservice.ServiceName = "identity.tokendecoder"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	var err error
	c.mgoSession, err = mgo.Dial(c.MgoAddr)
	if err != nil {
		return nil, err
	}
	logger.Info("Mongo", "Connected!")
	mgoCollection := c.mgoSession.DB(c.MgoDB).C(c.MgoColl)

	chConn := clickhouse.NewConn(c.ClickhouseAddr, clickhouse.NewHttpTransport())
	//"CREATE TABLE IF NOT EXISTS chats.messages (`time` DateTime('Asia/Yekaterinburg'),`chatid` String,`userid` String,`message` String,`type` Int) ENGINE = MergeTree() ORDER BY (time,chatid)"
	err = chConn.Ping()
	if err != nil {
		return nil, err
	}
	logger.Info("Clickhouse", "Connected!")
	return NewHandler(mgoCollection, connectors[tokenDecoderServiceName], chConn, c.ClickhouseTable)
}

func (conf *config) Close() error {

	conf.mgoSession.Close()
	return nil
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 5, &config{}, tokenDecoderServiceName)
}
