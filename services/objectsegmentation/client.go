package objectsegmentation

import (
	"bytes"
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/pointcloud"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/v1"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

// client is a client that implements the Object Segmentation Service.
type client struct {
	conn   rpc.ClientConn
	client pb.ObjectSegmentationServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	grpcClient := pb.NewObjectSegmentationServiceClient(conn)
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

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	return newSvcClientFromConn(conn, logger)
}

func (c *client) GetObjectPointClouds(ctx context.Context, cameraName string, params *vision.Parameters3D) ([]*vision.Object, error) {
	resp, err := c.client.GetObjectPointClouds(ctx, &pb.ObjectSegmentationServiceGetObjectPointCloudsRequest{
		Name:               cameraName,
		MimeType:           utils.MimeTypePCD,
		MinPointsInPlane:   int64(params.MinPtsInPlane),
		MinPointsInSegment: int64(params.MinPtsInSegment),
		ClusteringRadiusMm: params.ClusteringRadiusMm,
	})
	if err != nil {
		return nil, err
	}

	if resp.MimeType != utils.MimeTypePCD {
		return nil, fmt.Errorf("unknown pc mime type %s", resp.MimeType)
	}

	return protoToObjects(resp.Objects)
}

func protoToObjects(pco []*pb.PointCloudObject) ([]*vision.Object, error) {
	objects := make([]*vision.Object, len(pco))
	for i, o := range pco {
		pc, err := pointcloud.ReadPCD(bytes.NewReader(o.Frame))
		if err != nil {
			return nil, err
		}
		object := &vision.Object{
			PointCloud:  pc,
			Center:      protoToPoint(o.CenterCoordinatesMm),
			BoundingBox: protoToBox(o.BoundingBoxMm),
		}
		objects[i] = object
	}
	return objects, nil
}

func protoToPoint(p *commonpb.Vector3) pointcloud.Vec3 {
	return pointcloud.Vec3{p.X, p.Y, p.Z}
}

func protoToBox(b *commonpb.RectangularPrism) pointcloud.RectangularPrism {
	return pointcloud.RectangularPrism{b.WidthMm, b.LengthMm, b.DepthMm}
}
