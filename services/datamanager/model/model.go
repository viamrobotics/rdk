package model

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	v1 "go.viam.com/api/proto/viam/model/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	rdkutils "go.viam.com/rdk/utils"
)

// Manager is responsible for deploying model files.
type Manager interface {
	Deploy(ctx context.Context, req *v1.DeployRequest) (*v1.DeployResponse, error)
	Close()
}

// modelr is responsible for uploading files in captureDir to the cloud.
type modelr struct {
	partID            string
	conn              rpc.ClientConn
	client            v1.ModelServiceClient
	logger            golog.Logger
	backgroundWorkers sync.WaitGroup
	cancelCtx         context.Context
	cancelFunc        func()
}

// ManagerConstructor is a function for building a Manager.
type ManagerConstructor func(logger golog.Logger, cfg *config.Config) (Manager, error)

// NewDefaultManager returns the default Manager that syncs data to app.viam.com.
func NewDefaultManager(logger golog.Logger, cfg *config.Config) (Manager, error) {
	tlsConfig := config.NewTLSConfig(cfg).Config
	cloudConfig := cfg.Cloud
	rpcOpts := []rpc.DialOption{
		rpc.WithTLSConfig(tlsConfig),
		rpc.WithEntityCredentials(
			cloudConfig.ID,
			rpc.Credentials{
				Type:    rdkutils.CredentialsTypeRobotSecret,
				Payload: cloudConfig.Secret,
			}),
	}
	appAddress := "app.viam.com:443"

	conn, err := NewConnection(logger, appAddress, rpcOpts)
	if err != nil {
		return nil, err
	}
	client := NewClient(conn)
	return NewManager(logger, cfg.Cloud.ID, client, conn)
}

// NewManager returns a new modelr.
func NewManager(logger golog.Logger, partID string, client v1.ModelServiceClient,
	conn rpc.ClientConn,
) (Manager, error) {
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
	resp, err := s.client.Deploy(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Close closes all resources (goroutines) associated with s.
func (s *modelr) Close() {
	s.cancelFunc()
	s.backgroundWorkers.Wait()
	if s.conn != nil {
		if err := s.conn.Close(); err != nil {
			s.logger.Errorw("error closing datasync server connection", "error", err)
		}
	}
}
