package model

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"

	v1 "go.viam.com/api/proto/viam/model/v1"
	"go.viam.com/rdk/config"
	"go.viam.com/utils/rpc"
)

type Manager interface {
	Deploy(ctx context.Context, req *v1.DeployRequest) (*v1.DeployResponse, error)
	Close()
}

// syncer is responsible for uploading files in captureDir to the cloud.
type modelr struct {
	partID            string
	conn              rpc.ClientConn
	client            v1.ModelServiceClient
	logger            golog.Logger
	backgroundWorkers sync.WaitGroup
	cancelCtx         context.Context
	cancelFunc        func()
}

type ManagerConstructor func(logger golog.Logger, cfg *config.Config) (Manager, error)

// NewDefaultManager returns the default Manager that syncs data to app.viam.com.
func NewDefaultManager(logger golog.Logger, cfg *config.Config) (Manager, error) {
	fmt.Println("model/model.go/NewDefaultManager()")
	// tlsConfig := config.NewTLSConfig(cfg).Config
	// cloudConfig := cfg.Cloud
	// rpcOpts := []rpc.DialOption{
	// 	rpc.WithTLSConfig(tlsConfig),
	// 	rpc.WithEntityCredentials(
	// 		cloudConfig.ID,
	// 		rpc.Credentials{
	// 			Type:    rdkutils.CredentialsTypeRobotSecret,
	// 			Payload: cloudConfig.Secret,
	// 		}),
	// }
	rpcOpts := []rpc.DialOption{
		rpc.WithInsecure(),
	}
	var appAddress = ""
	if cfg.Cloud.AppAddress == "" {
		appAddress = "app.viam.com:443"
	} else {
		fmt.Println("we have set it properly")
		appAddress = cfg.Cloud.AppAddress
	}

	conn, err := NewConnection(logger, appAddress, rpcOpts)
	if err != nil {
		return nil, err
	}
	client := NewClient(conn)
	return NewManager(logger, cfg.Cloud.ID, client, conn)
}

// NewManager returns a new syncer.
func NewManager(logger golog.Logger, partID string, client v1.ModelServiceClient,
	conn rpc.ClientConn,
) (Manager, error) {
	fmt.Println("model/model.go/NewManager()")
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	ret := modelr{
		conn:              conn,
		client:            client,
		logger:            logger,
		backgroundWorkers: sync.WaitGroup{},
		cancelCtx:         cancelCtx,
		cancelFunc:        cancelFunc,
		partID:            partID,
	}
	return &ret, nil
}

func (s *modelr) Deploy(ctx context.Context, req *v1.DeployRequest) (*v1.DeployResponse, error) {
	return s.client.Deploy(ctx, req)
}

func (s *modelr) Close() {
	s.cancelFunc()
	s.backgroundWorkers.Wait()
	if s.conn != nil {
		if err := s.conn.Close(); err != nil {
			s.logger.Errorw("error closing datasync server connection", "error", err)
		}
	}
}
