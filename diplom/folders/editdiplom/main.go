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

const thisServiceName httpservice.ServiceName = "folders.editdiplom"
const tokenDecoderServiceName httpservice.ServiceName = "identity.tokendecoder"
const authGetServiceName httpservice.ServiceName = "auth.get"
const viewDiplomServiceName httpservice.ServiceName = "folders.viewdiplom"
const getMetausersServiceName httpservice.ServiceName = "folders.getmetausers"

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

	return NewHandler(templ, connectors[authGetServiceName], connectors[tokenDecoderServiceName], connectors[getMetausersServiceName], connectors[viewDiplomServiceName])
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 50, &config{}, tokenDecoderServiceName, authGetServiceName, viewDiplomServiceName, getMetausersServiceName)
}
