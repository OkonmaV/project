package main

import (
	"context"

	"thin-peak/httpservice"
)

type config struct {
	Configurator string
	Listen       string
}

const thisServiceName httpservice.ServiceName = "protocol.create"
const getuserdataServiceName httpservice.ServiceName = "identity.getuserdata"
const getquizresultsServiceName httpservice.ServiceName = "quiz.getquizresults"
const tokenDecoderServiceName httpservice.ServiceName = "identity.tokendecoder"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {

	return NewHandler(connectors[getuserdataServiceName], connectors[getquizresultsServiceName], connectors[tokenDecoderServiceName])
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 5, &config{}, getuserdataServiceName, getquizresultsServiceName, tokenDecoderServiceName)
}
