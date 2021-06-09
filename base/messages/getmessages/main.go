package main

import (
	"context"
	"thin-peak/httpservice"
)

type config struct {
	Configurator string
	Listen       string
	MgoDB        string
	MgoAddr      string
	MgoColl      string
}

var thisServiceName httpservice.ServiceName = "folders.getchats"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {

	return NewHandler(c.MgoDB, c.MgoAddr, c.MgoColl)
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 5, &config{})
}
