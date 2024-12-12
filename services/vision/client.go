package vision

import (
	"bytes"
	"context"
	"fmt"
	"image"

	"github.com/pkg/errors"
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
	"go.viam.com/rdk/vision/viscapture"
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
	return protoToDets(resp.Detections)
}

func (c *client) Detections(ctx context.Context, img image.Image, extra map[string]interface{},
) ([]objdet.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::Detections")
	defer span.End()
	if img == nil {
		return nil, errors.New("nil image input to given client.Detections")
	}
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
	return protoToDets(resp.Detections)
}

func protoToDets(protoDets []*pb.Detection) ([]objdet.Detection, error) {
	detections := make([]objdet.Detection, 0, len(protoDets))
	for _, d := range protoDets {
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
	return protoToClas(resp.Classifications), nil
}

func (c *client) Classifications(ctx context.Context, img image.Image,
	n int, extra map[string]interface{},
) (classification.Classifications, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::Classifications")
	defer span.End()
	if img == nil {
		return nil, errors.New("nil image input to given client.Classifications")
	}
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
	return protoToClas(resp.Classifications), nil
}

func protoToClas(protoClass []*pb.Classification) classification.Classifications {
	classifications := make([]classification.Classification, 0, len(protoClass))
	for _, c := range protoClass {
		classif := classification.NewClassification(c.Confidence, c.ClassName)
		classifications = append(classifications, classif)
	}
	return classifications
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

func (c *client) GetProperties(ctx context.Context, extra map[string]interface{}) (*Properties, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::GetProperties")
	defer span.End()

	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.GetProperties(ctx, &pb.GetPropertiesRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}

	return &Properties{resp.ClassificationsSupported, resp.DetectionsSupported, resp.ObjectPointCloudsSupported}, nil
}

func (c *client) CaptureAllFromCamera(
	ctx context.Context,
	cameraName string,
	captureOptions viscapture.CaptureOptions,
	extra map[string]interface{},
) (viscapture.VisCapture, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::ClassificationsFromCamera")
	defer span.End()
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return viscapture.VisCapture{}, err
	}
	resp, err := c.client.CaptureAllFromCamera(ctx, &pb.CaptureAllFromCameraRequest{
		Name:                    c.name,
		CameraName:              cameraName,
		ReturnImage:             captureOptions.ReturnImage,
		ReturnDetections:        captureOptions.ReturnDetections,
		ReturnClassifications:   captureOptions.ReturnClassifications,
		ReturnObjectPointClouds: captureOptions.ReturnObject,
		Extra:                   ext,
	})
	if err != nil {
		return viscapture.VisCapture{}, err
	}

	dets, err := protoToDets(resp.Detections)
	if err != nil {
		return viscapture.VisCapture{}, err
	}

	class := protoToClas(resp.Classifications)

	objPCD, err := protoToObjects(resp.Objects)
	if err != nil {
		return viscapture.VisCapture{}, err
	}

	var img image.Image
	if resp.Image.Image != nil {
		mimeType := utils.FormatToMimeType[resp.Image.GetFormat()]
		img, err = rimage.DecodeImage(ctx, resp.Image.Image, mimeType)
		if err != nil {
			return viscapture.VisCapture{}, err
		}
	}

	vcExtra := resp.Extra.AsMap()
	if len(vcExtra) == 0 {
		vcExtra = nil
	}

	capt := viscapture.VisCapture{
		Image:           img,
		Detections:      dets,
		Classifications: class,
		Objects:         objPCD,
		Extra:           vcExtra,
	}

	return capt, nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::client::DoCommand")
	defer span.End()

	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
