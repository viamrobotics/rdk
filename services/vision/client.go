package vision

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"go.opencensus.io/trace"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/service/vision/v1"
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
	ctx, span := trace.StartSpan(ctx, "service::objectdetection::client::DetectorNames")
	defer span.End()
	resp, err := c.client.DetectorNames(ctx, &pb.DetectorNamesRequest{})
	if err != nil {
		return nil, err
	}
	return resp.DetectorNames, nil
}

func (c *client) AddDetector(ctx context.Context, cfg DetectorConfig) (bool, error) {
	ctx, span := trace.StartSpan(ctx, "service::objectdetection::client::AddDetector")
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
	ctx, span := trace.StartSpan(ctx, "service::objectdetection::client::Detect")
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
