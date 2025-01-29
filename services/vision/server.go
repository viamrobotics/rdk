package vision

import (
	"bytes"
	"context"
	"image"

	"go.opencensus.io/trace"
	commonpb "go.viam.com/api/common/v1"
	camerapb "go.viam.com/api/component/camera/v1"
	pb "go.viam.com/api/service/vision/v1"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/pointcloud"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/viscapture"
)

// serviceServer implements the Vision Service.
type serviceServer struct {
	pb.UnimplementedVisionServiceServer
	coll resource.APIResourceCollection[Service]
}

// NewRPCServiceServer constructs a vision gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceCollection[Service]) interface{} {
	return &serviceServer{coll: coll}
}

func (server *serviceServer) GetDetections(
	ctx context.Context,
	req *pb.GetDetectionsRequest,
) (*pb.GetDetectionsResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::GetDetections")
	defer span.End()
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	img, err := rimage.DecodeImage(ctx, req.Image, req.MimeType)
	if err != nil {
		return nil, err
	}
	detections, err := svc.Detections(ctx, img, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetDetectionsResponse{
		Detections: detsToProto(detections),
	}, nil
}

func (server *serviceServer) GetDetectionsFromCamera(
	ctx context.Context,
	req *pb.GetDetectionsFromCameraRequest,
) (*pb.GetDetectionsFromCameraResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::GetDetectionsFromCamera")
	defer span.End()
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	detections, err := svc.DetectionsFromCamera(ctx, req.CameraName, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetDetectionsFromCameraResponse{
		Detections: detsToProto(detections),
	}, nil
}

func detsToProto(detections []objectdetection.Detection) []*pb.Detection {
	protoDets := make([]*pb.Detection, 0, len(detections))
	for _, det := range detections {
		box := det.BoundingBox()
		if box == nil {
			return nil
		}
		xMin := int64(box.Min.X)
		yMin := int64(box.Min.Y)
		xMax := int64(box.Max.X)
		yMax := int64(box.Max.Y)
		d := &pb.Detection{
			XMin:       &xMin,
			YMin:       &yMin,
			XMax:       &xMax,
			YMax:       &yMax,
			Confidence: det.Score(),
			ClassName:  det.Label(),
		}
		protoDets = append(protoDets, d)
	}
	return protoDets
}

func (server *serviceServer) GetClassifications(
	ctx context.Context,
	req *pb.GetClassificationsRequest,
) (*pb.GetClassificationsResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::GetClassifications")
	defer span.End()
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	img, err := rimage.DecodeImage(ctx, req.Image, req.MimeType)
	if err != nil {
		return nil, err
	}
	classifications, err := svc.Classifications(ctx, img, int(req.N), req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetClassificationsResponse{
		Classifications: clasToProto(classifications),
	}, nil
}

func (server *serviceServer) GetClassificationsFromCamera(
	ctx context.Context,
	req *pb.GetClassificationsFromCameraRequest,
) (*pb.GetClassificationsFromCameraResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::GetClassificationsFromCamera")
	defer span.End()
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	classifications, err := svc.ClassificationsFromCamera(ctx, req.CameraName, int(req.N), req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetClassificationsFromCameraResponse{
		Classifications: clasToProto(classifications),
	}, nil
}

func clasToProto(classifications classification.Classifications) []*pb.Classification {
	protoCs := make([]*pb.Classification, 0, len(classifications))
	for _, c := range classifications {
		cc := &pb.Classification{
			ClassName:  c.Label(),
			Confidence: c.Score(),
		}
		protoCs = append(protoCs, cc)
	}
	return protoCs
}

// GetObjectPointClouds returns an array of objects from the frame from a camera of the underlying robot. A specific MIME type
// can be requested but may not necessarily be the same one returned. Also returns a Vector3 array of the center points of each object.
func (server *serviceServer) GetObjectPointClouds(
	ctx context.Context,
	req *pb.GetObjectPointCloudsRequest,
) (*pb.GetObjectPointCloudsResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	objects, err := svc.GetObjectPointClouds(ctx, req.CameraName, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	protoSegments, err := segmentsToProto(req.CameraName, objects)
	if err != nil {
		return nil, err
	}

	return &pb.GetObjectPointCloudsResponse{
		MimeType: utils.MimeTypePCD,
		Objects:  protoSegments,
	}, nil
}

func segmentsToProto(frame string, segs []*vision.Object) ([]*commonpb.PointCloudObject, error) {
	protoSegs := make([]*commonpb.PointCloudObject, 0, len(segs))
	for _, seg := range segs {
		var buf bytes.Buffer
		if seg.PointCloud == nil {
			seg.PointCloud = pointcloud.New()
		}
		err := pointcloud.ToPCD(seg, &buf, pointcloud.PCDBinary)
		if err != nil {
			return nil, err
		}
		ps := &commonpb.PointCloudObject{
			PointCloud: buf.Bytes(),
			Geometries: &commonpb.GeometriesInFrame{
				Geometries:     []*commonpb.Geometry{seg.Geometry.ToProtobuf()},
				ReferenceFrame: frame,
			},
		}
		protoSegs = append(protoSegs, ps)
	}
	return protoSegs, nil
}

func (server *serviceServer) GetProperties(ctx context.Context,
	req *pb.GetPropertiesRequest,
) (*pb.GetPropertiesResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::GetProperties")
	defer span.End()
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	props, err := svc.GetProperties(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}

	out := &pb.GetPropertiesResponse{
		ClassificationsSupported:   props.ClassificationSupported,
		DetectionsSupported:        props.DetectionSupported,
		ObjectPointCloudsSupported: props.ObjectPCDsSupported,
	}
	return out, nil
}

func (server *serviceServer) CaptureAllFromCamera(
	ctx context.Context,
	req *pb.CaptureAllFromCameraRequest,
) (*pb.CaptureAllFromCameraResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::CaptureAllFromCamera")
	defer span.End()
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	captOptions := viscapture.CaptureOptions{
		ReturnImage:           req.ReturnImage,
		ReturnDetections:      req.ReturnDetections,
		ReturnClassifications: req.ReturnClassifications,
		ReturnObject:          req.ReturnObjectPointClouds,
	}

	capt, err := svc.CaptureAllFromCamera(ctx,
		req.CameraName,
		captOptions,
		req.Extra.AsMap(),
	)
	if err != nil {
		return nil, err
	}

	objProto, err := segmentsToProto(req.CameraName, capt.Objects)
	if err != nil {
		return nil, err
	}

	imgProto, err := imageToProto(ctx, capt.Image, req.CameraName)
	if err != nil {
		return nil, err
	}
	extraProto, err := protoutils.StructToStructPb(capt.Extra)
	if err != nil {
		return nil, err
	}
	return &pb.CaptureAllFromCameraResponse{
		Image:           imgProto,
		Detections:      detsToProto(capt.Detections),
		Classifications: clasToProto(capt.Classifications),
		Objects:         objProto,
		Extra:           extraProto,
	}, nil
}

func imageToProto(ctx context.Context, img image.Image, cameraName string) (*camerapb.Image, error) {
	if img == nil {
		return &camerapb.Image{}, nil
	}
	imgBytes, mimeType, err := encodeUnknownType(ctx, img, utils.MimeTypeJPEG)
	if err != nil {
		return nil, err
	}
	format := utils.MimeTypeToFormat[mimeType]
	return &camerapb.Image{
		Image:      imgBytes,
		Format:     format,
		SourceName: cameraName,
	}, nil
}

func encodeUnknownType(ctx context.Context, img image.Image, defaultMime string) ([]byte, string, error) {
	var mimeType string

	switch im := img.(type) {
	case *rimage.LazyEncodedImage:
		return im.RawData(), im.MIMEType(), nil
	case *image.Gray, *rimage.DepthMap:
		mimeType = utils.MimeTypeRawDepth

	default:
		mimeType = defaultMime
	}
	imgBytes, err := rimage.EncodeImage(ctx, img, mimeType)
	if err != nil {
		return nil, "", err
	}
	return imgBytes, mimeType, nil
}

// DoCommand receives arbitrary commands.
func (server *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::DoCommand")
	defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return rprotoutils.DoFromResourceServer(ctx, svc, req)
}
