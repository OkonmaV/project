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

type service struct {
	pub_service *httpservice.Publisher
}

const thisServiceName httpservice.ServiceName = "service2"
const publisherName httpservice.ServiceName = "service1"

func (c *config) CreateHandler(ctx context.Context, pubs_getter httpservice.Publishers_getter) (httpservice.HTTPService, error) {
	println(c.SayHello)
	s := &service{pub_service: pubs_getter.Get(publisherName)}
	return s, nil
}

func (s *service) Handle(req *suckhttp.Request, l types.Logger) (*suckhttp.Response, error) {
	if rereq, err := httpservice.CreateHTTPRequestFrom(suckhttp.POST, req); err != nil {
		panic(err.Error())
	} else {
		rereq.Body = append(rereq.Body, []byte(" UPDATED BY "+thisServiceName)...)
		resp, err := s.pub_service.SendHTTP(rereq)
		if err != nil {
			println(err.Error())
		} else {
			return resp, nil
		}
		return nil, err
	}
}

func main() {
	httpservice.InitNewService(thisServiceName, &config{}, false, 1, publisherName)
}
