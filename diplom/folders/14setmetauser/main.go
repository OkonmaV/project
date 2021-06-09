package main

import (
	"context"
	"thin-peak/httpservice"
)

type config struct {
	Configurator     string
	Listen           string
	MgoDB            string
	MgoAddr          string
	MgoColl          string
	MgoCollMetausers string
}

var thisServiceName httpservice.ServiceName = "folders.setmetauser"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {

	return NewSetMetaUser(c.MgoDB, c.MgoAddr, c.MgoColl, c.MgoCollMetausers)
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 5, &config{})
}
