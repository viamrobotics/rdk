package slam

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"

	"github.com/edaniels/golog"
	"go.opencensus.io/trace"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/pointcloud"
	pb "go.viam.com/rdk/proto/api/service/slam/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

// client is a client that satisfies the slam.proto contract.
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

// NewClientFromConn constructs a new Client from the connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	return newSvcClientFromConn(conn, logger)
}

// GetPosition creates a request, calls the slam service GetPosition, and parses the response into the desired PoseInFrame.
func (c *client) GetPosition(ctx context.Context, name string) (*referenceframe.PoseInFrame, error) {
	ctx, span := trace.StartSpan(ctx, "slam::client::GetPosition")
	defer span.End()

	req := &pb.GetPositionRequest{
		Name: name,
	}

	resp, err := c.client.GetPosition(ctx, req)
	if err != nil {
		return nil, err
	}
	p := resp.GetPose()
	return referenceframe.ProtobufToPoseInFrame(p), nil
}

// GetMap creates a request, calls the slam service GetMap, and parses the response into the desired mimeType and map data.
func (c *client) GetMap(ctx context.Context, name, mimeType string, cameraPosition *referenceframe.PoseInFrame, includeRobotMarker bool) (
	string, image.Image, *vision.Object, error,
) {
	ctx, span := trace.StartSpan(ctx, "slam::client::GetMap")
	defer span.End()

	req := &pb.GetMapRequest{
		Name:               name,
		MimeType:           mimeType,
		CameraPosition:     referenceframe.PoseInFrameToProtobuf(cameraPosition).Pose,
		IncludeRobotMarker: includeRobotMarker,
	}

	var imageData image.Image
	vObject := &vision.Object{}

	resp, err := c.client.GetMap(ctx, req)
	if err != nil {
		return "", imageData, vObject, err
	}

	mimeType = resp.MimeType

	switch mimeType {
	case utils.MimeTypeJPEG:
		_, span_decode := trace.StartSpan(ctx, "slam::client::Decode::")
		defer span_decode.End()

		imData := resp.GetImage()
		imageData, err = jpeg.Decode(bytes.NewReader(imData))
		if err != nil {
			return "", imageData, vObject, err
		}
	case utils.MimeTypePCD:
		_, span_decode := trace.StartSpan(ctx, "slam::client::GetPointCloud::")
		defer span_decode.End()

		pcData := resp.GetPointCloud()
		pc, err := pointcloud.ReadPCD(bytes.NewReader(pcData.PointCloud))
		if err != nil {
			return "", imageData, vObject, err
		}
		vObject, err = vision.NewObject(pc)
		if err != nil {
			return "", imageData, vObject, err
		}
	}

	return mimeType, imageData, vObject, nil
}
