// Package generic contains a gRPC based generic client.
package generic

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	pb "go.viam.com/rdk/proto/api/component/generic/v1"

	"google.golang.org/protobuf/types/known/structpb"
)

// serviceClient is a client satisfies the generic.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.GenericServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewGenericServiceClient(conn)
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

// client is a Generic client.
type client struct {
	*serviceClient
	name string
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Generic {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) Generic {
	return &client{sc, name}
}

func (c *client) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return DoFromConnection(ctx, c.conn, c.name, cmd)
}

// DoFromConnection is a helper to allow Do() calls from other component clients.
func DoFromConnection(ctx context.Context, conn rpc.ClientConn, name string, cmd map[string]interface{}) (map[string]interface{}, error) {
	gclient := pb.NewGenericServiceClient(conn)
	command, err := structpb.NewStruct(cmd)
	if err != nil {
		return nil, err
	}

	resp, err := gclient.Do(ctx, &pb.DoRequest{
		Name:    name,
		Command: command,
	})
	if err != nil {
		return nil, err
	}
	return resp.Result.AsMap(), nil
}
