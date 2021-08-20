package main

import (
	"context"
	"fmt"
	"thin-peak/httpservice"
)

type config struct {
	Configurator string
	Listen       string
	AuthFilePath string
	AuthKeyLen   int
	AuthValueLen int
}

const thisServiceName httpservice.ServiceName = "identity.logout"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {

	return NewHandler()
}

func main() {
	// if _, err := toml.DecodeFile("config.toml", &conf); err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	// fmt.Println("config: ", conf)
	//----

	fmt.Println("started")
	httpservice.InitNewService(thisServiceName, false, 5, &config{})
}
