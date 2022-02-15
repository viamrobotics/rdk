// Package forcematrix contains a gRPC based forcematrix client.
package forcematrix

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/v1"
)

// serviceClient satisfies the force_matrix.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.ForceMatrixServiceClient
	logger golog.Logger
}

// newServiceClient constructs a new serviceClient that is served at the given address.
func newServiceClient(ctx context.Context, address string, logger golog.Logger, opts ...rpc.DialOption) (*serviceClient, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewForceMatrixServiceClient(conn)
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

// Close cleanly closes the underlying connections.
func (sc *serviceClient) Close() error {
	return sc.conn.Close()
}

var _ = sensor.Sensor(&client{})

// client is a ForceMatrix client.
type client struct {
	*serviceClient
	name string
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, logger golog.Logger, opts ...rpc.DialOption) (ForceMatrix, error) {
	sc, err := newServiceClient(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	return clientFromSvcClient(sc, name), nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) ForceMatrix {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) ForceMatrix {
	return &client{sc, name}
}

func (c *client) ReadMatrix(ctx context.Context) ([][]int, error) {
	resp, err := c.client.ReadMatrix(ctx,
		&pb.ForceMatrixServiceReadMatrixRequest{
			Name: c.name,
		})
	if err != nil {
		return nil, err
	}
	return protoToMatrix(resp), nil
}

func (c *client) DetectSlip(ctx context.Context) (bool, error) {
	resp, err := c.client.DetectSlip(ctx,
		&pb.ForceMatrixServiceDetectSlipRequest{
			Name: c.name,
		})
	if err != nil {
		return false, err
	}
	return resp.SlipDetected, nil
}

func (c *client) GetReadings(ctx context.Context) ([]interface{}, error) {
	return GetReadings(ctx, c)
}

// Close cleanly closes the underlying connections.
func (c *client) Close() error {
	return c.serviceClient.Close()
}

// protoToMatrix is a helper function to convert protobuf matrix values into a 2-dimensional int slice.
func protoToMatrix(matrixResponse *pb.ForceMatrixServiceReadMatrixResponse) [][]int {
	numRows := matrixResponse.Matrix.Rows
	numCols := matrixResponse.Matrix.Cols

	matrix := make([][]int, numRows)
	for row := range matrix {
		matrix[row] = make([]int, numCols)
		for col := range matrix[row] {
			matrix[row][col] = int(matrixResponse.Matrix.Data[row*int(numCols)+col])
		}
	}
	return matrix
}
