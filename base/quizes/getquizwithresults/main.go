package main

import (
	"context"
	"project/base/quizes/repo"
	"thin-peak/httpservice"

	"github.com/big-larry/mgo"
)

type config struct {
	Configurator   string
	Listen         string
	MgoDB          string
	MgoAddr        string
	MgoColl        string
	MgoCollResults string
	mgoSession     *mgo.Session
}

const thisServiceName httpservice.ServiceName = "quiz.getquizwithresults"
const tokenDecoderServiceName httpservice.ServiceName = "identity.tokendecoder"
const authGetServiceName httpservice.ServiceName = "auth.get"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	templ, err := repo.GetTemplate("index.html")
	if err != nil {
		return nil, err
	}
	mgosession, col, err := repo.ConnectToMongo(c.MgoAddr, c.MgoDB, c.MgoColl)
	if err != nil {
		return nil, err
	}
	c.mgoSession = mgosession

	return NewHandler(templ, col, c.mgoSession.DB(c.MgoDB).C(c.MgoCollResults), connectors[authGetServiceName], connectors[tokenDecoderServiceName])
}

func (conf *config) Close() error {
	conf.mgoSession.Close()
	return nil
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 5, &config{}, tokenDecoderServiceName, authGetServiceName)
}
