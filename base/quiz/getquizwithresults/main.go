package main

import (
	"context"
	"text/template"

	"io/ioutil"
	"thin-peak/httpservice"
)

type config struct {
	Configurator string
	Listen       string
}

const thisServiceName httpservice.ServiceName = "quiz.getquizwithresults"
const getQuizServiceName httpservice.ServiceName = "quiz.getquiz"
const getQuizResultsServiceName httpservice.ServiceName = "quiz.getquizresults"
const tokenDecoderServiceName httpservice.ServiceName = "identity.tokendecoder"
const authGetServiceName httpservice.ServiceName = "auth.get"

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
	return NewHandler(templ, connectors[authGetServiceName], connectors[tokenDecoderServiceName], connectors[getQuizServiceName], connectors[getQuizResultsServiceName])
}

func main() {
	httpservice.InitNewService(thisServiceName, false, 50, &config{}, tokenDecoderServiceName, authGetServiceName, getQuizServiceName, getQuizResultsServiceName)
}