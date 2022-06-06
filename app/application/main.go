package main

import (
	"context"
	"project/logs/logger"
	"project/wsservice"
	"sync"
)

type config struct {
}

type service struct {
	path     string
	users    []*wsconn
	messages []message
	sync.RWMutex
}

func (s *service) addconn(wsc *wsconn) {
	s.Lock()
	defer s.Unlock()
	s.users = append(s.users, wsc)
}

func (s *service) deleteconn(wsc *wsconn) {
	s.Lock()
	defer s.Unlock()
	for i := 0; i < len(s.users); i++ {
		if s.users[i] == wsc {
			s.users = append(s.users[:i], s.users[i+1:]...)
			return
		}
	}
}

func (s *service) addmessage(m message) {
	s.Lock()
	defer s.Unlock()

	s.messages = append(s.messages, m)
}

func (s *service) getmessages() []message {
	s.RLock()
	defer s.RUnlock()

	msgs := make([]message, 0, len(s.messages))
	for _, m := range s.messages {
		msgs = append(msgs, m)
	}

	return msgs
}

const thisServiceName wsservice.ServiceName = "test.application"

func (c *config) CreateService(ctx context.Context, pubs_getter wsservice.Publishers_getter) (wsservice.WSService, error) {
	return &service{messages: make([]message, 0), users: make([]*wsconn, 0)}, nil
}

// wsservice.Service interface implementation
func (s *service) CreateNewWsHandler(l logger.Logger) wsservice.Handler {
	return &wsconn{
		l:    l,
		srvc: s,
	}
}

// wsservice.closer interface implementation
func (s *service) Close() error {
	return nil
}

func main() {
	wsservice.InitNewServiceWithoutConfigurator(thisServiceName, &config{}, 1)
}
