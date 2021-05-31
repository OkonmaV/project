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

var thisServiceName httpservice.ServiceName = "conf.createverifyemail"
var emailVerifyServiceName httpservice.ServiceName = "conf.emailverify"
var userRegistrationServiceName httpservice.ServiceName = "conf.userregistration"
var setUserDataServiceName httpservice.ServiceName = "conf.setuserdata"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	return NewCreateVerifyEmail(c.TrntlAddr, c.TrntlTable, connectors[emailVerifyServiceName], connectors[userRegistrationServiceName], connectors[setUserDataServiceName])
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 5, &config{}, emailVerifyServiceName, userRegistrationServiceName, setUserDataServiceName)
}
