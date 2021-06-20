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

const thisServiceName httpservice.ServiceName = "webrtc.broadcast"
const tokenDecoderServiceName httpservice.ServiceName = "identity.tokendecoder"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	return NewHandler(c.Memcached, connectors[tokenDecoderServiceName])
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 50, &config{}, tokenDecoderServiceName)
}
