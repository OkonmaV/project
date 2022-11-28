package main

import (
	"context"
	identityserver "project/app/identityservice"
	"project/app/protocol"
	"project/logs/logger"
	"sync"
)

const thisServiceName = "test.idserv"

type service struct{}

type idservice struct {
	clients map[string]*client
	cmux    sync.Mutex
	users   map[string]*user
	umux    sync.Mutex

	l logger.Logger
}

type client struct {
}
type user struct {
}

func (srvc *service) CreateHandler(ctx context.Context, l logger.Logger, pgttr identityserver.Publishers_getter) (identityserver.Handler, error) {
	return &idservice{
		clients: make(map[string]*client),
		users:   make(map[string]*user),
		l:       l,
	}, nil
}

func (idsrvc *idservice) Handle_Token_Request(client_id, secret, code, redirect_uri string) (accessttoken, refreshtoken, userid string, expires int64, errCode protocol.ErrorCode) {
	s
}
func (idsrvc *idservice) Handle_AppAuth(appname, appid string) (errCode protocol.ErrorCode) {

}
func (idsrvc *idservice) Handle_AppRegistration(appname string) (appid, secret string, errCode protocol.ErrorCode) {

}
func (idsrvc *idservice) Handle_Auth_Request(login, password string, client_id string, redirect_uri string, scope []string) (grant_code string, errCode protocol.ErrorCode) {

}
func (idsrvc *idservice) Handle_UserRegistration_Request(login, password string) (errCode protocol.ErrorCode) {

}
func main() {
	identityserver.InitNewService(thisServiceName)
}
