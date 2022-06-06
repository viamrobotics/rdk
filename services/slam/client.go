package slam

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/slam/v1"
	"go.viam.com/rdk/utils"
)

// client is a client satisfies the slam.proto contract.
type client struct {
	conn   rpc.ClientConn
	client pb.SLAMServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	grpcClient := pb.NewSLAMServiceClient(conn)
	sc := &client{
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return sc
}

// Close cleanly closes the underlying connections.
func (c *client) Close() error {
	return c.conn.Close()
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, logger golog.Logger, opts ...rpc.DialOption) (Service, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
}

// NewClientFromConn constructs a new Client from the connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	return newSvcClientFromConn(conn, logger)
}

// GetPosition client side, creates request calls slam service function GetPosition and parses the response.
func (c *client) GetPosition(ctx context.Context, name string) (*commonpb.PoseInFrame, error) {
	req := &pb.GetPositionRequest{
		Name: name,
	}

	resp, err := c.client.GetPosition(ctx, req)
	if err != nil {
		return nil, err
	}
	p := resp.GetPose()
	return p, nil
}

// GetMap client side, creates request calls slam service function GetMap and parses the response.
func (c *client) GetMap(ctx context.Context, name, mimeType string, cameraPosition *commonpb.Pose, includeRobotMarker bool) (
	string, []byte, *commonpb.PointCloudObject, error) {
	req := &pb.GetMapRequest{
		Name:               name,
		MimeType:           mimeType,
		CameraPosition:     cameraPosition,
		IncludeRobotMarker: includeRobotMarker,
	}

	resp, err := c.client.GetMap(ctx, req)
	if err != nil {
		return "", []byte{}, &commonpb.PointCloudObject{}, err
	}

	mimeType = resp.MimeType

	imageData := []byte{}
	pcData := &commonpb.PointCloudObject{}
	switch mimeType {
	case utils.MimeTypeJPEG:
		imageData = resp.GetImage()
	case utils.MimeTypePCD:
		pcData = resp.GetPointCloud()
	}

	return mimeType, imageData, pcData, nil
}
