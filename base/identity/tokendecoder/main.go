package main

import (
	"context"
	"thin-peak/httpservice"
)

type config struct {
	Configurator string
	Listen       string
	JwtKey       string
	Cookie       string
}

var thisServiceName httpservice.ServiceName = "identity.tokendecoder"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	return NewTokenDecoder(c.JwtKey, c.Cookie)
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 10, &config{})
}
