package vision

import (
	"bytes"
	"context"
	"fmt"
	"image"

	"go.opencensus.io/trace"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/vision/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

// client implements VisionServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.VisionServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Service, error) {
	grpcClient := pb.NewVisionServiceClient(conn)
	c := &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		client: grpcClient,
		logger: logger,
	}
	return c, nil
}

func (c *client) DetectionsFromCamera(
	ctx context.Context,
	cameraName string,
	extra map[string]interface{},
) ([]objdet.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::DetectionsFromCamera")
	defer span.End()
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetDetectionsFromCamera(ctx, &pb.GetDetectionsFromCameraRequest{
		Name:       c.name,
		CameraName: cameraName,
		Extra:      ext,
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

func (c *client) Detections(ctx context.Context, img image.Image, extra map[string]interface{},
) ([]objdet.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::Detections")
	defer span.End()
	mimeType := gostream.MIMETypeHint(ctx, utils.MimeTypeJPEG)
	imgBytes, err := rimage.EncodeImage(ctx, img, mimeType)
	if err != nil {
		return nil, err
	}
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetDetections(ctx, &pb.GetDetectionsRequest{
		Name:     c.name,
		Image:    imgBytes,
		Width:    int64(img.Bounds().Dx()),
		Height:   int64(img.Bounds().Dy()),
		MimeType: mimeType,
		Extra:    ext,
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

func (c *client) ClassificationsFromCamera(
	ctx context.Context,
	cameraName string,
	n int,
	extra map[string]interface{},
) (classification.Classifications, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::ClassificationsFromCamera")
	defer span.End()
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetClassificationsFromCamera(ctx, &pb.GetClassificationsFromCameraRequest{
		Name:       c.name,
		CameraName: cameraName,
		N:          int32(n),
		Extra:      ext,
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
	n int, extra map[string]interface{},
) (classification.Classifications, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::Classifications")
	defer span.End()
	mimeType := gostream.MIMETypeHint(ctx, utils.MimeTypeJPEG)
	imgBytes, err := rimage.EncodeImage(ctx, img, mimeType)
	if err != nil {
		return nil, err
	}
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetClassifications(ctx, &pb.GetClassificationsRequest{
		Name:     c.name,
		Image:    imgBytes,
		Width:    int32(img.Bounds().Dx()),
		Height:   int32(img.Bounds().Dy()),
		MimeType: mimeType,
		N:        int32(n),
		Extra:    ext,
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

func (c *client) GetObjectPointClouds(
	ctx context.Context,
	cameraName string,
	extra map[string]interface{},
) ([]*vision.Object, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetObjectPointClouds(ctx, &pb.GetObjectPointCloudsRequest{
		Name:       c.name,
		CameraName: cameraName,
		MimeType:   utils.MimeTypePCD,
		Extra:      ext,
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
		if len(o.Geometries.Geometries) >= 1 {
			objects[i], err = vision.NewObjectWithLabel(pc, label, o.Geometries.GetGeometries()[0])
		} else {
			objects[i], err = vision.NewObjectWithLabel(pc, label, nil)
		}
		if err != nil {
			return nil, err
		}
	}
	return objects, nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::DoCommand")
	defer span.End()

	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
