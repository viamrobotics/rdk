package vision

import (
	"bytes"
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"go.opencensus.io/trace"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/pointcloud"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/vision/v1"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

// client is a client that implements the Object Detection Service.
type client struct {
	conn   rpc.ClientConn
	client pb.VisionServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	grpcClient := pb.NewVisionServiceClient(conn)
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

func (c *client) DetectorNames(ctx context.Context) ([]string, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::DetectorNames")
	defer span.End()
	resp, err := c.client.DetectorNames(ctx, &pb.DetectorNamesRequest{})
	if err != nil {
		return nil, err
	}
	return resp.DetectorNames, nil
}

func (c *client) AddDetector(ctx context.Context, cfg DetectorConfig) (bool, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::AddDetector")
	defer span.End()
	params, err := structpb.NewStruct(cfg.Parameters)
	if err != nil {
		return false, err
	}
	resp, err := c.client.AddDetector(ctx, &pb.AddDetectorRequest{
		DetectorName:       cfg.Name,
		DetectorModelType:  cfg.Type,
		DetectorParameters: params,
	})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}

func (c *client) Detect(ctx context.Context, cameraName, detectorName string) ([]objdet.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::Detect")
	defer span.End()
	resp, err := c.client.Detect(ctx, &pb.DetectRequest{
		CameraName:   cameraName,
		DetectorName: detectorName,
	})
	if err != nil {
		return nil, err
	}
	detections := make([]objdet.Detection, 0, len(resp.Detections))
	for _, d := range resp.Detections {
		box := image.Rect(int(d.XMin), int(d.YMin), int(d.XMax), int(d.YMax))
		det := objdet.NewDetection(box, d.Confidence, d.ClassName)
		detections = append(detections, det)
	}
	return detections, nil
}

func (c *client) GetSegmenters(ctx context.Context) ([]string, error) {
	resp, err := c.client.GetSegmenters(ctx, &pb.GetSegmentersRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Segmenters, nil
}

func (c *client) GetSegmenterParameters(ctx context.Context, segmenterName string) ([]utils.TypedName, error) {
	resp, err := c.client.GetSegmenterParameters(ctx, &pb.GetSegmenterParametersRequest{
		SegmenterName: segmenterName,
	})
	if err != nil {
		return nil, err
	}
	params := make([]utils.TypedName, len(resp.Parameters))
	for i, p := range resp.Parameters {
		params[i] = utils.TypedName{p.Name, p.Type}
	}
	return params, nil
}

func (c *client) GetObjectPointClouds(ctx context.Context,
	cameraName string,
	segmenterName string,
	params config.AttributeMap,
) ([]*vision.Object, error) {
	conf, err := structpb.NewStruct(params)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetObjectPointClouds(ctx, &pb.GetObjectPointCloudsRequest{
		CameraName:    cameraName,
		SegmenterName: segmenterName,
		MimeType:      utils.MimeTypePCD,
		Parameters:    conf,
	})
	if err != nil {
		return nil, err
	}

	if resp.MimeType != utils.MimeTypePCD {
		return nil, fmt.Errorf("unknown pc mime type %s", resp.MimeType)
	}
	return protoToObjects(resp.Objects)
}

func protoToObjects(pco []*commonpb.PointCloudObject) ([]*vision.Object, error) {
	objects := make([]*vision.Object, len(pco))
	for i, o := range pco {
		pc, err := pointcloud.ReadPCD(bytes.NewReader(o.PointCloud))
		if err != nil {
			return nil, err
		}
		objects[i], err = vision.NewObject(pc)
		if err != nil {
			return nil, err
		}
	}
	return objects, nil
}
