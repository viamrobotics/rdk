package vision

import (
	"bytes"
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"go.opencensus.io/trace"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/vision/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

// client implements VisionServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.VisionServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	grpcClient := pb.NewVisionServiceClient(conn)
	c := &client{
		name:   name,
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return c
}

func (c *client) GetDetectorNames(ctx context.Context) ([]string, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::GetDetectorNames")
	defer span.End()
	resp, err := c.client.GetDetectorNames(ctx, &pb.GetDetectorNamesRequest{Name: c.name})
	if err != nil {
		return nil, err
	}
	return resp.DetectorNames, nil
}

func (c *client) AddDetector(ctx context.Context, cfg DetectorConfig) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::AddDetector")
	defer span.End()
	params, err := protoutils.StructToStructPb(cfg.Parameters)
	if err != nil {
		return err
	}
	_, err = c.client.AddDetector(ctx, &pb.AddDetectorRequest{
		Name:               c.name,
		DetectorName:       cfg.Name,
		DetectorModelType:  cfg.Type,
		DetectorParameters: params,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) GetDetectionsFromCamera(ctx context.Context, cameraName, detectorName string) ([]objdet.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::GetDetectionsFromCamera")
	defer span.End()
	resp, err := c.client.GetDetectionsFromCamera(ctx, &pb.GetDetectionsFromCameraRequest{
		Name:         c.name,
		CameraName:   cameraName,
		DetectorName: detectorName,
	})
	if err != nil {
		return nil, err
	}
	detections := make([]objdet.Detection, 0, len(resp.Detections))
	for _, d := range resp.Detections {
		if d.XMin == nil || d.XMax == nil || d.YMin == nil || d.YMax == nil {
			return nil, fmt.Errorf("invalid detection %+v", d)
		}
		box := image.Rect(int(*d.XMin), int(*d.YMin), int(*d.XMax), int(*d.YMax))
		det := objdet.NewDetection(box, d.Confidence, d.ClassName)
		detections = append(detections, det)
	}
	return detections, nil
}

func (c *client) GetDetections(ctx context.Context, img image.Image, detectorName string,
) ([]objdet.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::GetDetections")
	defer span.End()
	mimeType := utils.MimeTypePNG
	imgBytes, err := rimage.EncodeImage(ctx, img, mimeType)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetDetections(ctx, &pb.GetDetectionsRequest{
		Name:         c.name,
		Image:        imgBytes,
		Width:        int64(img.Bounds().Dx()),
		Height:       int64(img.Bounds().Dy()),
		MimeType:     mimeType,
		DetectorName: detectorName,
	})
	if err != nil {
		return nil, err
	}
	detections := make([]objdet.Detection, 0, len(resp.Detections))
	for _, d := range resp.Detections {
		if d.XMin == nil || d.XMax == nil || d.YMin == nil || d.YMax == nil {
			return nil, fmt.Errorf("invalid detection %+v", d)
		}
		box := image.Rect(int(*d.XMin), int(*d.YMin), int(*d.XMax), int(*d.YMax))
		det := objdet.NewDetection(box, d.Confidence, d.ClassName)
		detections = append(detections, det)
	}
	return detections, nil
}

func (c *client) GetSegmenterNames(ctx context.Context) ([]string, error) {
	resp, err := c.client.GetSegmenterNames(ctx, &pb.GetSegmenterNamesRequest{Name: c.name})
	if err != nil {
		return nil, err
	}
	return resp.SegmenterNames, nil
}

func (c *client) GetSegmenterParameters(ctx context.Context, segmenterName string) ([]utils.TypedName, error) {
	resp, err := c.client.GetSegmenterParameters(ctx, &pb.GetSegmenterParametersRequest{
		Name:          c.name,
		SegmenterName: segmenterName,
	})
	if err != nil {
		return nil, err
	}
	params := make([]utils.TypedName, len(resp.SegmenterParameters))
	for i, p := range resp.SegmenterParameters {
		params[i] = utils.TypedName{p.Name, p.Type}
	}
	return params, nil
}

func (c *client) GetObjectPointClouds(ctx context.Context,
	cameraName string,
	segmenterName string,
	params config.AttributeMap,
) ([]*vision.Object, error) {
	conf, err := protoutils.StructToStructPb(params)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetObjectPointClouds(ctx, &pb.GetObjectPointCloudsRequest{
		Name:          c.name,
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
