package main

import (
	"context"
	"html/template"
	"io/ioutil"

	"thin-peak/httpservice"
)

type config struct {
	Configurator string
	Listen       string
}

const thisServiceName httpservice.ServiceName = "protocol.create"
const getuserdataServiceName httpservice.ServiceName = "identity.getuserdata"
const getquizresultsServiceName httpservice.ServiceName = "quiz.getquizwithresults"
const getFoldersServiceName httpservice.ServiceName = "folders.getfolders"
const tokenDecoderServiceName httpservice.ServiceName = "identity.tokendecoder"

func (c *config) GetListenAddress() string {
	return c.Listen
}
func (c *config) GetConfiguratorAddress() string {
	return c.Configurator
}
func (c *config) CreateHandler(ctx context.Context, connectors map[httpservice.ServiceName]*httpservice.InnerService) (httpservice.HttpService, error) {
	templData, err := ioutil.ReadFile("index.html")
	if err != nil {
		return nil, err
	}

	templ, err := template.New("index").Parse(string(templData))
	if err != nil {
		return nil, err
	}

	return NewHandler(templ, connectors[tokenDecoderServiceName], connectors[getuserdataServiceName], connectors[getquizresultsServiceName], connectors[getFoldersServiceName])
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 50, &config{}, getuserdataServiceName, getquizresultsServiceName, tokenDecoderServiceName, getFoldersServiceName)
}
