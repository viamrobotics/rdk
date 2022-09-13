package vision

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/invopop/jsonschema"
	"go.opencensus.io/trace"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/pointcloud"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/vision/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
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

func (c *client) GetModelParameterSchema(ctx context.Context, modelType VisModelType) (*jsonschema.Schema, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::GetModelParameterSchema")
	defer span.End()
	resp, err := c.client.GetModelParameterSchema(ctx, &pb.GetModelParameterSchemaRequest{Name: c.name, ModelType: string(modelType)})
	if err != nil {
		return nil, err
	}
	outp := &jsonschema.Schema{}
	err = json.Unmarshal(resp.ModelParameterSchema, outp)
	if err != nil {
		return nil, err
	}
	return outp, nil
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

func (c *client) AddDetector(ctx context.Context, cfg VisModelConfig) error {
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

func (c *client) RemoveDetector(ctx context.Context, detectorName string) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::RemoveDetector")
	defer span.End()
	_, err := c.client.RemoveDetector(ctx, &pb.RemoveDetectorRequest{
		Name:         c.name,
		DetectorName: detectorName,
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
	mimeType := utils.MimeTypeRawRGBA
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

func (c *client) GetClassifierNames(ctx context.Context) ([]string, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::GetClassifierNames")
	defer span.End()
	resp, err := c.client.GetClassifierNames(ctx, &pb.GetClassifierNamesRequest{Name: c.name})
	if err != nil {
		return nil, err
	}
	return resp.ClassifierNames, nil
}

func (c *client) AddClassifier(ctx context.Context, cfg VisModelConfig) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::AddClassifier")
	defer span.End()
	params, err := protoutils.StructToStructPb(cfg.Parameters)
	if err != nil {
		return err
	}
	_, err = c.client.AddClassifier(ctx, &pb.AddClassifierRequest{
		Name:                 c.name,
		ClassifierName:       cfg.Name,
		ClassifierModelType:  cfg.Type,
		ClassifierParameters: params,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) RemoveClassifier(ctx context.Context, classifierName string) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::RemoveClassifier")
	defer span.End()
	_, err := c.client.RemoveClassifier(ctx, &pb.RemoveClassifierRequest{
		Name:           c.name,
		ClassifierName: classifierName,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) GetClassificationsFromCamera(ctx context.Context, cameraName,
	classifierName string, n int,
) (classification.Classifications, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::GetClassificationsFromCamera")
	defer span.End()
	resp, err := c.client.GetClassificationsFromCamera(ctx, &pb.GetClassificationsFromCameraRequest{
		Name:           c.name,
		CameraName:     cameraName,
		ClassifierName: classifierName,
		N:              int32(n),
	})
	if err != nil {
		return nil, err
	}
	classifications := make([]classification.Classification, 0, len(resp.Classifications))
	for _, c := range resp.Classifications {
		classif := classification.NewClassification(c.Confidence, c.ClassName)
		classifications = append(classifications, classif)
	}
	return classifications, nil
}

func (c *client) GetClassifications(ctx context.Context, img image.Image,
	classifierName string, n int,
) (classification.Classifications, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::GetClassifications")
	defer span.End()
	mimeType := utils.MimeTypeRawRGBA
	imgBytes, err := rimage.EncodeImage(ctx, img, mimeType)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetClassifications(ctx, &pb.GetClassificationsRequest{
		Name:           c.name,
		Image:          imgBytes,
		Width:          int32(img.Bounds().Dx()),
		Height:         int32(img.Bounds().Dy()),
		MimeType:       mimeType,
		ClassifierName: classifierName,
		N:              int32(n),
	})
	if err != nil {
		return nil, err
	}
	classifications := make([]classification.Classification, 0, len(resp.Classifications))
	for _, c := range resp.Classifications {
		classif := classification.NewClassification(c.Confidence, c.ClassName)
		classifications = append(classifications, classif)
	}
	return classifications, nil
}

func (c *client) GetSegmenterNames(ctx context.Context) ([]string, error) {
	resp, err := c.client.GetSegmenterNames(ctx, &pb.GetSegmenterNamesRequest{Name: c.name})
	if err != nil {
		return nil, err
	}
	return resp.SegmenterNames, nil
}

func (c *client) AddSegmenter(ctx context.Context, cfg VisModelConfig) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::AddSegmenter")
	defer span.End()
	params, err := protoutils.StructToStructPb(cfg.Parameters)
	if err != nil {
		return err
	}
	_, err = c.client.AddSegmenter(ctx, &pb.AddSegmenterRequest{
		Name:                c.name,
		SegmenterName:       cfg.Name,
		SegmenterModelType:  cfg.Type,
		SegmenterParameters: params,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) RemoveSegmenter(ctx context.Context, segmenterName string) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::RemoveSegmenter")
	defer span.End()
	_, err := c.client.RemoveSegmenter(ctx, &pb.RemoveSegmenterRequest{
		Name:          c.name,
		SegmenterName: segmenterName,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) GetObjectPointClouds(ctx context.Context,
	cameraName string,
	segmenterName string,
) ([]*vision.Object, error) {
	resp, err := c.client.GetObjectPointClouds(ctx, &pb.GetObjectPointCloudsRequest{
		Name:          c.name,
		CameraName:    cameraName,
		SegmenterName: segmenterName,
		MimeType:      utils.MimeTypePCD,
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
