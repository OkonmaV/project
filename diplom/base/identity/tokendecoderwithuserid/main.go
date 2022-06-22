package main

import (
	"context"
	"thin-peak/httpservice"
)

type config struct {
	Configurator string
	Listen       string
	JwtKey       string
	CookieName   string
	TrntlAddr    string
	TrntlTable   string
}

var thisServiceName httpservice.ServiceName = "identity.tokendecoderwithuserid"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	return NewHandler(c.TrntlAddr, c.TrntlTable, c.JwtKey, c.CookieName)
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 10, &config{})
}
