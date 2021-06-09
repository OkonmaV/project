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
	MgoDB        string
	MgoAddr      string
	MgoColl      string
}

var thisServiceName httpservice.ServiceName = "folders.createmetauser"
var codegenerationServiceName httpservice.ServiceName = "identity.codegeneration"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	return NewCreateMetauser(c.TrntlAddr, c.TrntlTable, c.MgoDB, c.MgoAddr, c.MgoColl, connectors[codegenerationServiceName])
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 5, &config{}, codegenerationServiceName)
}
