package main

import (
	"context"

	"thin-peak/httpservice"
)

type config struct {
	Configurator       string
	Listen             string
	TrntlAddr          string
	TrntlTable         string
	TrntlTableRegcodes string
}

var thisServiceName httpservice.ServiceName = "identity.registrationbycodeemailverify"
var verifyServiceName httpservice.ServiceName = "identity.verify"
var userRegistrationServiceName httpservice.ServiceName = "identity.userregistration"
var setUserDataServiceName httpservice.ServiceName = "identity.setuserdata"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	return NewCreateVerifyEmail(c.TrntlAddr, c.TrntlTable, c.TrntlTableRegcodes, connectors[verifyServiceName], connectors[userRegistrationServiceName], connectors[setUserDataServiceName])
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 5, &config{}, verifyServiceName, userRegistrationServiceName, setUserDataServiceName)
}
