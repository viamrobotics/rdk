// Package model implements model storage/deployment client.
package model

import (
	"context"
	"fmt"
	"reflect"

	// "sync"

	// "github.com/pkg/errors"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	v1 "go.viam.com/api/proto/viam/model/v1"
)

// client implements ModelServiceClient.
type client struct {
	conn   rpc.ClientConn
	client v1.ModelServiceClient
	logger golog.Logger
}

// NewClient constructs a new pb.ModelServiceClient using the passed in connection.
func NewClient(conn rpc.ClientConn) v1.ModelServiceClient {
	fmt.Println("model/client.go/NewClient()")
	return v1.NewModelServiceClient(conn)
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	fmt.Println("model/client.go/newSvcClientFromConn()")
	grpcClient := NewClient(conn)
	sc := &client{
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return sc
}

// NewClientFromConn constructs a new Client from connection passed in.
//nolint:revive
func NewClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	fmt.Println("model/client.go/NewClientFromConn()")
	return newSvcClientFromConn(conn, logger)
}

func (c *client) Delete(ctx context.Context, req *v1.DeleteRequest) (*v1.DeleteResponse, error) {
	return c.client.Delete(ctx, req)
}

func (c *client) Upload(ctx context.Context) (v1.ModelService_UploadClient, error) {
	return c.client.Upload(ctx)
}

func (c *client) Deploy(ctx context.Context, req *v1.DeployRequest) (*v1.DeployResponse, error) {
	fmt.Println("model/client.go/Deploy()")
	fmt.Println(reflect.TypeOf(c))
	fmt.Println(reflect.TypeOf(c.client))

	return c.client.Deploy(ctx, req)
}

// // =-----
// type Manager interface {
// 	Deploy(ctx context.Context, req *v1.DeployRequest)
// }

// // syncer is responsible for uploading files in captureDir to the cloud.
// type syncer struct {
// 	partID            string
// 	conn              rpc.ClientConn
// 	client            v1.DataSyncServiceClient
// 	logger            golog.Logger
// 	progressTracker   progressTracker
// 	backgroundWorkers sync.WaitGroup
// 	cancelCtx         context.Context
// 	cancelFunc        func()
// }

// func NewManager(logger golog.Logger, partID string, client v1.ModelServiceClient,
// 	conn rpc.ClientConn,
// ) (Manager, error) {
// 	fmt.Println("datasync/sync.go/NewManager()")
// 	cancelCtx, cancelFunc := context.WithCancel(context.Background())
// 	ret := syncer{
// 		conn:   conn,
// 		client: client,
// 		logger: logger,
// 		progressTracker: progressTracker{
// 			lock:        &sync.Mutex{},
// 			m:           make(map[string]struct{}),
// 			progressDir: viamProgressDotDir,
// 		},
// 		backgroundWorkers: sync.WaitGroup{},
// 		cancelCtx:         cancelCtx,
// 		cancelFunc:        cancelFunc,
// 		partID:            partID,
// 	}
// 	if err := ret.progressTracker.initProgressDir(); err != nil {
// 		return nil, errors.Wrap(err, "couldn't initialize progress tracking directory")
// 	}
// 	return &ret, nil
// }
