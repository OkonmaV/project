package main

import (
	"context"
	"thin-peak/httpservice"
)

type config struct {
	Configurator string
	Listen       string
	MgoDB        string
	MgoAddr      string
	MgoColl      string
}

const thisServiceName httpservice.ServiceName = "identity.createmanyusersbylist"
const tokenDecoderServiceName httpservice.ServiceName = "identity.tokendecoder"
const authGetServiceName httpservice.ServiceName = "auth.get"
const userRegistrationServiceName httpservice.ServiceName = "identity.userregistration"
const setUserDataServiceName httpservice.ServiceName = "identity.setuserdata"
const createMetauserServiceName httpservice.ServiceName = "folders.createonlymetauser"
const createFolderWithMetauser httpservice.ServiceName = "folders.createfolderwithmetauser"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	return NewHandler(connectors[tokenDecoderServiceName], connectors[authGetServiceName], connectors[userRegistrationServiceName], connectors[setUserDataServiceName], connectors[createMetauserServiceName], connectors[createFolderWithMetauser])
}

func main() {
	httpservice.InitNewService(
		thisServiceName,
		false,
		5,
		&config{},
		tokenDecoderServiceName,
		authGetServiceName,
		userRegistrationServiceName,
		setUserDataServiceName,
		createMetauserServiceName,
		createFolderWithMetauser)
}
