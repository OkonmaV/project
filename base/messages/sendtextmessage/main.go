package main

import (
	"context"
	"thin-peak/httpservice"
)

type config struct {
	Configurator    string
	Listen          string
	MgoAddr         string
	MgoColl         string
	ClickhouseAddr  string
	ClickhouseTable string
}

var thisServiceName httpservice.ServiceName = "conf.sendtextmessage"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {

	return NewSendMessage(c.MgoAddr, c.MgoColl, c.ClickhouseAddr, c.ClickhouseTable)
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 5, &config{})
}
