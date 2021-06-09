package main

import (
	"context"
	"thin-peak/httpservice"
)

type config struct {
	Configurator string
	Listen       string
	TrntlAddr    string
	TrntlTable   string
}

var thisServiceName httpservice.ServiceName = "identity.authentication"
var tokenGenServiceName httpservice.ServiceName = "identity.tokengenerator"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {

	return NewAuthentication(c.TrntlAddr, c.TrntlTable, connectors[tokenGenServiceName])
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 5, &config{}, tokenGenServiceName)
}
