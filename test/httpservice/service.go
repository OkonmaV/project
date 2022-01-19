package httpservice

import (
	"context"
	"project/test/types"

	"github.com/big-larry/suckhttp"
)

type Configier interface {
	GetConfiguratorAddress() string
}

type Servicier interface {
	InitServiceData(ctx context.Context) ([]string, error) // хер знает как это говно назвать
	Handle(request *suckhttp.Request, logger *types.Logger) (*suckhttp.Response, error)
}

func InitNewService(servicename ServiceName, keepConnAlive bool, maxConnections int, publishers ...ServiceName) {
}
