package main

import (
	"context"

	"thin-peak/httpservice"
)

const serviceName = httpservice.ServiceName("files.fs.upload")

type config struct {
	Configurator string
	Listen       string
	Path         string
}

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	auth, err := httpservice.NewAuthorizer(serviceName, connectors[httpservice.ServiceName("auth.get")], connectors[httpservice.ServiceName("identity.tokendecoder")])
	if err != nil {
		return nil, err
	}
	return NewHandler(c.Path, auth)
}

func main() {
	httpservice.InitNewService(serviceName, false, 10, &config{}, httpservice.ServiceName("auth.get"), httpservice.ServiceName("identity.tokendecoder"))
}
