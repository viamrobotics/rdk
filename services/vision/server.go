package vision

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/vision/v1"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

// subtypeServer implements the Vision Service.
type subtypeServer struct {
	pb.UnimplementedVisionServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a vision gRPC service server.
func NewServer(s subtype.Service) pb.VisionServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service(serviceName string) (Service, error) {
	resource := server.subtypeSvc.Resource(serviceName)
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Named(serviceName))
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("vision.Service", resource)
	}
	return svc, nil
}

func (server *subtypeServer) GetDetectorNames(
	ctx context.Context,
	req *pb.GetDetectorNamesRequest,
) (*pb.GetDetectorNamesResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::GetDetectorNames")
	defer span.End()
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	names, err := svc.GetDetectorNames(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetDetectorNamesResponse{
		DetectorNames: names,
	}, nil
}

func (server *subtypeServer) AddDetector(
	ctx context.Context,
	req *pb.AddDetectorRequest,
) (*pb.AddDetectorResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::AddDetector")
	defer span.End()
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	params := config.AttributeMap(req.DetectorParameters.AsMap())
	cfg := DetectorConfig{
		Name:       req.DetectorName,
		Type:       req.DetectorModelType,
		Parameters: params,
	}
	err = svc.AddDetector(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &pb.AddDetectorResponse{}, nil
}

func (server *subtypeServer) GetDetections(
	ctx context.Context,
	req *pb.GetDetectionsRequest,
) (*pb.GetDetectionsResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::GetDetections")
	defer span.End()
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	img, err := rimage.DecodeImage(ctx, req.Image, req.MimeType, int(req.Width), int(req.Height))
	if err != nil {
		return nil, err
	}
	detections, err := svc.GetDetections(ctx, img, req.DetectorName)
	if err != nil {
		return nil, err
	}
	protoDets := make([]*pb.Detection, 0, len(detections))
	for _, det := range detections {
		box := det.BoundingBox()
		if box == nil {
			return nil, errors.New("detection has no bounding box, must return a bounding box")
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
	return &pb.GetDetectionsResponse{
		Detections: protoDets,
	}, nil
}

func (server *subtypeServer) GetDetectionsFromCamera(
	ctx context.Context,
	req *pb.GetDetectionsFromCameraRequest,
) (*pb.GetDetectionsFromCameraResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::GetDetectionsFromCamera")
	defer span.End()
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	detections, err := svc.GetDetectionsFromCamera(ctx, req.CameraName, req.DetectorName)
	if err != nil {
		return nil, err
	}
	protoDets := make([]*pb.Detection, 0, len(detections))
	for _, det := range detections {
		box := det.BoundingBox()
		if box == nil {
			return nil, errors.New("detection has no bounding box, must return a bounding box")
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
	return &pb.GetDetectionsFromCameraResponse{
		Detections: protoDets,
	}, nil
}

func (server *subtypeServer) GetSegmenterNames(
	ctx context.Context,
	req *pb.GetSegmenterNamesRequest,
) (*pb.GetSegmenterNamesResponse, error) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	names, err := svc.GetSegmenterNames(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetSegmenterNamesResponse{
		SegmenterNames: names,
	}, nil
}

func (server *subtypeServer) GetSegmenterParameters(
	ctx context.Context,
	req *pb.GetSegmenterParametersRequest,
) (*pb.GetSegmenterParametersResponse, error) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	params, err := svc.GetSegmenterParameters(ctx, req.SegmenterName)
	if err != nil {
		return nil, err
	}
	typedParams := make([]*pb.TypedParameter, len(params))
	for i, p := range params {
		typedParams[i] = &pb.TypedParameter{Name: p.Name, Type: p.Type}
	}
	return &pb.GetSegmenterParametersResponse{
		SegmenterParameters: typedParams,
	}, nil
}

// GetObjectPointClouds returns an array of objects from the frame from a camera of the underlying robot. A specific MIME type
// can be requested but may not necessarily be the same one returned. Also returns a Vector3 array of the center points of each object.
func (server *subtypeServer) GetObjectPointClouds(
	ctx context.Context,
	req *pb.GetObjectPointCloudsRequest,
) (*pb.GetObjectPointCloudsResponse, error) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	conf := config.AttributeMap(req.Parameters.AsMap())
	objects, err := svc.GetObjectPointClouds(ctx, req.CameraName, req.SegmenterName, conf)
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
		err := pointcloud.ToPCD(seg, &buf, pointcloud.PCDBinary)
		if err != nil {
			return nil, err
		}
		ps := &commonpb.PointCloudObject{
			PointCloud: buf.Bytes(),
			Geometries: &commonpb.GeometriesInFrame{
				Geometries:     []*commonpb.Geometry{seg.BoundingBox.ToProtobuf()},
				ReferenceFrame: frame,
			},
		}
		protoSegs = append(protoSegs, ps)
	}
	return protoSegs, nil
}
