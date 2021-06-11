package main

import (
	"context"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"

	"github.com/big-larry/mgo"
)

type config struct {
	Configurator     string
	Listen           string
	MgoDB            string
	MgoAddr          string
	MgoColl          string
	MgoCollMetausers string
	mgoSession       *mgo.Session
}

const thisServiceName httpservice.ServiceName = "folders.setmetauser"
const tokenDecoderServiceName httpservice.ServiceName = "identity.tokendecoder"
const authGetServiceName httpservice.ServiceName = "auth.get"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	mgoSession, err := mgo.Dial(c.MgoAddr)
	if err != nil {
		logger.Error("Mongo conn", err)
		return nil, err
	}
	logger.Info("Mongo", "Connected!")
	mgoCollection := mgoSession.DB(c.MgoDB).C(c.MgoColl)
	mgoCollectionMetausers := mgoSession.DB(c.MgoDB).C(c.MgoCollMetausers)
	return NewHandler(mgoCollection, mgoCollectionMetausers, connectors[authGetServiceName], connectors[tokenDecoderServiceName])
}

func (conf *config) Close() error {
	conf.mgoSession.Close()
	return nil
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 5, &config{})
}
