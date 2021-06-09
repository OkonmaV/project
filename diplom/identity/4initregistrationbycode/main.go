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

var thisServiceName httpservice.ServiceName = "identity.initregistrationbycode"
var createVerifyServiceName httpservice.ServiceName = "identity.createverify"
var createMailServiceName httpservice.ServiceName = "identity.createmail"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	return NewHandler(c.TrntlAddr, c.TrntlTable, connectors[createVerifyServiceName], connectors[createMailServiceName])
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 5, &config{}, createVerifyServiceName, createMailServiceName)
}
