package main

import (
	"context"

	"thin-peak/httpservice"
)

type config struct {
	Configurator string
	Listen       string
}

var thisServiceName httpservice.ServiceName = "identity.createmail"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	return NewHandler(connectors[httpservice.ServiceName("email.addtosend")])
}

func main() {
	httpservice.InitNewService(thisServiceName, true, 5, &config{}, httpservice.ServiceName("email.addtosend"))
}
