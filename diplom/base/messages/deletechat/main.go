package main

import (
	"context"
	"project/base/messages/repo"

	"thin-peak/httpservice"

	"github.com/big-larry/mgo"
)

type config struct {
	Configurator      string
	Listen            string
	MgoDB             string
	MgoAddr           string
	MgoColl           string
	MgoCollForDeleted string
	mgoSession        *mgo.Session
}

const thisServiceName httpservice.ServiceName = "messages.deletechat"
const tokenDecoderServiceName httpservice.ServiceName = "identity.tokendecoder"

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
	return NewHandler(col, c.mgoSession.DB(c.MgoDB).C(c.MgoCollForDeleted), connectors[tokenDecoderServiceName])
}

func (c *config) Close() error {
	c.mgoSession.Close()
	return nil
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 5, &config{}, tokenDecoderServiceName)
}
