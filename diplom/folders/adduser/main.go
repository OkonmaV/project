package main

import (
	"context"
	"project/diplom/folders/repo"
	"thin-peak/httpservice"

	"github.com/big-larry/mgo"
)

type config struct {
	Configurator string
	Listen       string
	MgoDB        string
	MgoDBUsers   string
	MgoAddr      string
	MgoColl      string
	MgoCollUsers string
	mgoSession   *mgo.Session
}

const thisServiceName httpservice.ServiceName = "folders.adduser"
const tokenDecoderServiceName httpservice.ServiceName = "identity.tokendecoder"
const authGetServiceName httpservice.ServiceName = "auth.get"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {

	mgosession, col, err := repo.ConnectToMongo(c.MgoAddr, c.MgoDB, c.MgoColl)
	if err != nil {
		return nil, err
	}
	c.mgoSession = mgosession
	return NewHandler(col, c.mgoSession.DB(c.MgoDBUsers).C(c.MgoCollUsers), connectors[authGetServiceName], connectors[tokenDecoderServiceName])
}

func (conf *config) Close() error {
	conf.mgoSession.Close()
	return nil
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 50, &config{}, tokenDecoderServiceName, authGetServiceName)
}
