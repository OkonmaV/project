package main

import (
	"context"
	"thin-peak/httpservice"
)

type config struct {
	Configurator string
	Listen       string
	Memcached    string
}

var thisServiceName httpservice.ServiceName = "webrtc.addbroadcast"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	return NewHandler(c.Memcached)
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 50, &config{})
}
