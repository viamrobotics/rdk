// Package camera contains a gRPC based camera client.
package camera

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"github.com/xfmoulet/qoi"
	"go.opencensus.io/trace"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/pointcloud"
	pb "go.viam.com/rdk/proto/api/component/camera/v1"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

// serviceClient is a client satisfies the camera.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.CameraServiceClient
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
	client := pb.NewCameraServiceClient(conn)
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

// client is an camera client.
type client struct {
	*serviceClient
	name string
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, logger golog.Logger, opts ...rpc.DialOption) (Camera, error) {
	sc, err := newServiceClient(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	return clientFromSvcClient(sc, name), nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Camera {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) Camera {
	return &client{sc, name}
}

func (c *client) Next(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::client::Next")
	defer span.End()
	resp, err := c.client.GetFrame(ctx, &pb.GetFrameRequest{
		Name:     c.name,
		MimeType: utils.MimeTypeViamBest,
	})
	if err != nil {
		return nil, nil, err
	}
	_, span2 := trace.StartSpan(ctx, "camera::client::Next::Decode::"+resp.MimeType)
	defer span2.End()
	switch resp.MimeType {
	case utils.MimeTypeRawRGBA:
		img := image.NewNRGBA(image.Rect(0, 0, int(resp.WidthPx), int(resp.HeightPx)))
		img.Pix = resp.Image
		return img, func() {}, nil
	case utils.MimeTypeRawIWD:
		img, err := rimage.ImageWithDepthFromRawBytes(int(resp.WidthPx), int(resp.HeightPx), resp.Image)
		return img, func() {}, err
	case utils.MimeTypeRawDepth:
		depth, err := rimage.ReadDepthMap(bufio.NewReader(bytes.NewReader(resp.Image)))
		img := rimage.MakeImageWithDepth(rimage.ConvertImage(depth.ToPrettyPicture(0, 0)), depth, true)
		return img, func() {}, err
	case utils.MimeTypeJPEG:
		img, err := jpeg.Decode(bytes.NewReader(resp.Image))
		return img, func() {}, err
	case utils.MimeTypePNG:
		img, err := png.Decode(bytes.NewReader(resp.Image))
		return img, func() {}, err
	case utils.MimeTypeQOI:
		img, err := qoi.Decode(bytes.NewReader(resp.Image))
		return img, func() {}, err
	default:
		return nil, nil, errors.Errorf("do not how to decode MimeType %s", resp.MimeType)
	}
}

func (c *client) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "camera-client::NextPointCloud")
	defer span.End()

	ctx, getPcdSpan := trace.StartSpan(ctx, "camera-client::NextPointCloud::GetPointCloud")
	resp, err := c.client.GetPointCloud(ctx, &pb.GetPointCloudRequest{
		Name:     c.name,
		MimeType: utils.MimeTypePCD,
	})
	getPcdSpan.End()
	if err != nil {
		return nil, err
	}

	if resp.MimeType != utils.MimeTypePCD {
		return nil, fmt.Errorf("unknown pc mime type %s", resp.MimeType)
	}

	return func() (pointcloud.PointCloud, error) {
		_, span := trace.StartSpan(ctx, "camera-client::NextPointCloud::ReadPCD")
		defer span.End()

		return pointcloud.ReadPCD(bytes.NewReader(resp.PointCloud))
	}()
}

// Close cleanly closes the underlying connections.
func (c *client) Close() error {
	return c.serviceClient.Close()
}

func (c *client) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}
