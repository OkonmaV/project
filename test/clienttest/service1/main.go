package main

import (
	"context"
	"project/test/httpservice"
	"project/test/types"

	"github.com/big-larry/suckhttp"
)

type config struct {
	SayHello string
}

type service struct{}

const thisServiceName httpservice.ServiceName = "service1"

//const publisherName httpservice.ServiceName = "service2"

func (c *config) CreateHandler(ctx context.Context, pubs_getter httpservice.Publishers_getter) (httpservice.HTTPService, error) {
	println(c.SayHello)
	s := &service{}
	return s, nil
}

func (s *service) Handle(req *suckhttp.Request, l types.Logger) (*suckhttp.Response, error) {
	println("new message ", string(req.Body), " from ", req.GetHeader("From"))
	return suckhttp.NewResponse(200, "OK").SetBody([]byte("recieved")), nil
}

func (s *service) Close() {
	println("ok, im closing")
}

func main() {
	httpservice.InitNewService(thisServiceName, &config{}, false, 1)
}
