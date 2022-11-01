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
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/vision/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/pointcloud"
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

func (c *client) GetModelParameterSchema(
	ctx context.Context,
	modelType VisModelType,
	extra map[string]interface{},
) (*jsonschema.Schema, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::GetModelParameterSchema")
	defer span.End()
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetModelParameterSchema(
		ctx,
		&pb.GetModelParameterSchemaRequest{Name: c.name, ModelType: string(modelType), Extra: ext},
	)
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

func (c *client) DetectorNames(ctx context.Context, extra map[string]interface{}) ([]string, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::DetectorNames")
	defer span.End()
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetDetectorNames(ctx, &pb.GetDetectorNamesRequest{Name: c.name, Extra: ext})
	if err != nil {
		return nil, err
	}
	return resp.DetectorNames, nil
}

func (c *client) AddDetector(ctx context.Context, cfg VisModelConfig, extra map[string]interface{}) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::AddDetector")
	defer span.End()
	params, err := protoutils.StructToStructPb(cfg.Parameters)
	if err != nil {
		return err
	}
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.AddDetector(ctx, &pb.AddDetectorRequest{
		Name:               c.name,
		DetectorName:       cfg.Name,
		DetectorModelType:  cfg.Type,
		DetectorParameters: params,
		Extra:              ext,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) RemoveDetector(ctx context.Context, detectorName string, extra map[string]interface{}) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::RemoveDetector")
	defer span.End()
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.RemoveDetector(ctx, &pb.RemoveDetectorRequest{
		Name:         c.name,
		DetectorName: detectorName,
		Extra:        ext,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) DetectionsFromCamera(
	ctx context.Context,
	cameraName, detectorName string,
	extra map[string]interface{},
) ([]objdet.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::DetectionsFromCamera")
	defer span.End()
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetDetectionsFromCamera(ctx, &pb.GetDetectionsFromCameraRequest{
		Name:         c.name,
		CameraName:   cameraName,
		DetectorName: detectorName,
		Extra:        ext,
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

func (c *client) Detections(ctx context.Context, img image.Image, detectorName string, extra map[string]interface{},
) ([]objdet.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::Detections")
	defer span.End()
	mimeType := utils.MimeTypeRawRGBA
	imgBytes, err := rimage.EncodeImage(ctx, img, mimeType)
	if err != nil {
		return nil, err
	}
	ext, err := protoutils.StructToStructPb(extra)
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
		Extra:        ext,
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

func (c *client) ClassifierNames(ctx context.Context, extra map[string]interface{}) ([]string, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::ClassifierNames")
	defer span.End()
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetClassifierNames(ctx, &pb.GetClassifierNamesRequest{Name: c.name, Extra: ext})
	if err != nil {
		return nil, err
	}
	return resp.ClassifierNames, nil
}

func (c *client) AddClassifier(ctx context.Context, cfg VisModelConfig, extra map[string]interface{}) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::AddClassifier")
	defer span.End()
	params, err := protoutils.StructToStructPb(cfg.Parameters)
	if err != nil {
		return err
	}
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.AddClassifier(ctx, &pb.AddClassifierRequest{
		Name:                 c.name,
		ClassifierName:       cfg.Name,
		ClassifierModelType:  cfg.Type,
		ClassifierParameters: params,
		Extra:                ext,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) RemoveClassifier(ctx context.Context, classifierName string, extra map[string]interface{}) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::RemoveClassifier")
	defer span.End()
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.RemoveClassifier(ctx, &pb.RemoveClassifierRequest{
		Name:           c.name,
		ClassifierName: classifierName,
		Extra:          ext,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) ClassificationsFromCamera(ctx context.Context, cameraName,
	classifierName string, n int, extra map[string]interface{},
) (classification.Classifications, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::ClassificationsFromCamera")
	defer span.End()
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetClassificationsFromCamera(ctx, &pb.GetClassificationsFromCameraRequest{
		Name:           c.name,
		CameraName:     cameraName,
		ClassifierName: classifierName,
		N:              int32(n),
		Extra:          ext,
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

func (c *client) Classifications(ctx context.Context, img image.Image,
	classifierName string, n int, extra map[string]interface{},
) (classification.Classifications, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::Classifications")
	defer span.End()
	mimeType := utils.MimeTypeRawRGBA
	imgBytes, err := rimage.EncodeImage(ctx, img, mimeType)
	if err != nil {
		return nil, err
	}
	ext, err := protoutils.StructToStructPb(extra)
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
		Extra:          ext,
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

func (c *client) SegmenterNames(ctx context.Context, extra map[string]interface{}) ([]string, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetSegmenterNames(ctx, &pb.GetSegmenterNamesRequest{Name: c.name, Extra: ext})
	if err != nil {
		return nil, err
	}
	return resp.SegmenterNames, nil
}

func (c *client) AddSegmenter(ctx context.Context, cfg VisModelConfig, extra map[string]interface{}) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::AddSegmenter")
	defer span.End()
	params, err := protoutils.StructToStructPb(cfg.Parameters)
	if err != nil {
		return err
	}
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.AddSegmenter(ctx, &pb.AddSegmenterRequest{
		Name:                c.name,
		SegmenterName:       cfg.Name,
		SegmenterModelType:  cfg.Type,
		SegmenterParameters: params,
		Extra:               ext,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) RemoveSegmenter(ctx context.Context, segmenterName string, extra map[string]interface{}) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::RemoveSegmenter")
	defer span.End()
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.RemoveSegmenter(ctx, &pb.RemoveSegmenterRequest{
		Name:          c.name,
		SegmenterName: segmenterName,
		Extra:         ext,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) GetObjectPointClouds(ctx context.Context,
	cameraName string,
	segmenterName string, extra map[string]interface{},
) ([]*vision.Object, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetObjectPointClouds(ctx, &pb.GetObjectPointCloudsRequest{
		Name:          c.name,
		CameraName:    cameraName,
		SegmenterName: segmenterName,
		MimeType:      utils.MimeTypePCD,
		Extra:         ext,
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
		// Sets the label to the first non-empty label of any geometry; defaults to the empty string.
		label := func() string {
			for _, g := range o.Geometries.GetGeometries() {
				if g.GetLabel() != "" {
					return g.GetLabel()
				}
			}
			return ""
		}()
		objects[i], err = vision.NewObjectWithLabel(pc, label)
		if err != nil {
			return nil, err
		}
	}
	return objects, nil
}
