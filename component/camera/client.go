// Package camera contains a gRPC based camera client.
package camera

import (
	"bytes"
	"context"
	"fmt"
	"image"

	rpcclient "go.viam.com/utils/rpc/client"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc/dialer"

	"go.viam.com/core/grpc"
	"go.viam.com/core/pointcloud"
	pb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/rimage"
)

// serviceClient is a client satisfies the camera.proto contract.
type serviceClient struct {
	conn   dialer.ClientConn
	client pb.CameraServiceClient
	logger golog.Logger
}

// newServiceClient constructs a new serviceClient that is served at the given address.
func newServiceClient(ctx context.Context, address string, opts rpcclient.DialOptions, logger golog.Logger) (*serviceClient, error) {
	conn, err := grpc.Dial(ctx, address, opts, logger)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn dialer.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewCameraServiceClient(conn)
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

// Close cleanly closes the underlying connections
func (sc *serviceClient) Close() error {
	return sc.conn.Close()
}

// client is an camera client
type client struct {
	*serviceClient
	name string
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, opts rpcclient.DialOptions, logger golog.Logger) (Camera, error) {
	sc, err := newServiceClient(ctx, address, opts, logger)
	if err != nil {
		return nil, err
	}
	return clientFromSvcClient(sc, name), nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(conn dialer.ClientConn, name string, logger golog.Logger) Camera {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) Camera {
	return &client{sc, name}
}

func (c *client) Next(ctx context.Context) (image.Image, func(), error) {
	resp, err := c.client.Frame(ctx, &pb.CameraServiceFrameRequest{
		Name:     c.name,
		MimeType: grpc.MimeTypeViamBest,
	})
	if err != nil {
		return nil, nil, err
	}
	switch resp.MimeType {
	case grpc.MimeTypeRawRGBA:
		img := image.NewNRGBA(image.Rect(0, 0, int(resp.DimX), int(resp.DimY)))
		img.Pix = resp.Frame
		return img, func() {}, nil
	case grpc.MimeTypeRawIWD:
		img, err := rimage.ImageWithDepthFromRawBytes(int(resp.DimX), int(resp.DimY), resp.Frame)
		return img, func() {}, err
	default:
		return nil, nil, errors.Errorf("do not how to decode MimeType %s", resp.MimeType)
	}

}

func (c *client) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	resp, err := c.client.PointCloud(ctx, &pb.CameraServicePointCloudRequest{
		Name:     c.name,
		MimeType: grpc.MimeTypePCD,
	})
	if err != nil {
		return nil, err
	}

	if resp.MimeType != grpc.MimeTypePCD {
		return nil, fmt.Errorf("unknown pc mime type %s", resp.MimeType)
	}

	return pointcloud.ReadPCD(bytes.NewReader(resp.Frame))
}

// Close cleanly closes the underlying connections
func (c *client) Close() error {
	return c.serviceClient.Close()
}
