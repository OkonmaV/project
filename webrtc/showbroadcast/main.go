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
	Memcached    string
}

const thisServiceName httpservice.ServiceName = "webrtc.showbroadcast"
const tokenDecoderServiceName httpservice.ServiceName = "identity.tokendecoder"
const authGetServiceName httpservice.ServiceName = "auth.get"
const getFoldersServiceName httpservice.ServiceName = "folders.getfolders"
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

	return NewHandler(c.Memcached, templ, connectors[authGetServiceName], connectors[tokenDecoderServiceName], connectors[getFoldersServiceName], connectors[getMetausersServiceName])
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 50, &config{}, tokenDecoderServiceName, authGetServiceName, getFoldersServiceName, getMetausersServiceName)
}
