package vision

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/vision/v1"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
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
		return nil, NewUnimplementedInterfaceError(resource)
	}
	return svc, nil
}

func (server *subtypeServer) GetModelParameterSchema(
	ctx context.Context,
	req *pb.GetModelParameterSchemaRequest,
) (*pb.GetModelParameterSchemaResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::GetModelParameterSchema")
	defer span.End()
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	params, err := svc.GetModelParameterSchema(ctx, VisModelType(req.ModelType), req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return &pb.GetModelParameterSchemaResponse{
		ModelParameterSchema: data,
	}, nil
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
	names, err := svc.DetectorNames(ctx, req.Extra.AsMap())
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
	cfg := VisModelConfig{
		Name:       req.DetectorName,
		Type:       req.DetectorModelType,
		Parameters: params,
	}
	err = svc.AddDetector(ctx, cfg, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.AddDetectorResponse{}, nil
}

func (server *subtypeServer) RemoveDetector(
	ctx context.Context,
	req *pb.RemoveDetectorRequest,
) (*pb.RemoveDetectorResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::RemoveDetector")
	defer span.End()
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	err = svc.RemoveDetector(ctx, req.DetectorName, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.RemoveDetectorResponse{}, nil
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
	img, err := rimage.DecodeImage(ctx, req.Image, req.MimeType)
	if err != nil {
		return nil, err
	}
	detections, err := svc.Detections(ctx, img, req.DetectorName, req.Extra.AsMap())
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
	detections, err := svc.DetectionsFromCamera(ctx, req.CameraName, req.DetectorName, req.Extra.AsMap())
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

func (server *subtypeServer) GetClassifierNames(
	ctx context.Context,
	req *pb.GetClassifierNamesRequest,
) (*pb.GetClassifierNamesResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::GetClassifierNames")
	defer span.End()
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	names, err := svc.ClassifierNames(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetClassifierNamesResponse{
		ClassifierNames: names,
	}, nil
}

func (server *subtypeServer) AddClassifier(
	ctx context.Context,
	req *pb.AddClassifierRequest,
) (*pb.AddClassifierResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::AddClassifier")
	defer span.End()
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	params := config.AttributeMap(req.ClassifierParameters.AsMap())
	cfg := VisModelConfig{
		Name:       req.ClassifierName,
		Type:       req.ClassifierModelType,
		Parameters: params,
	}
	err = svc.AddClassifier(ctx, cfg, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.AddClassifierResponse{}, nil
}

func (server *subtypeServer) RemoveClassifier(
	ctx context.Context,
	req *pb.RemoveClassifierRequest,
) (*pb.RemoveClassifierResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::RemoveClassifier")
	defer span.End()
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	err = svc.RemoveDetector(ctx, req.ClassifierName, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.RemoveClassifierResponse{}, nil
}

func (server *subtypeServer) GetClassifications(
	ctx context.Context,
	req *pb.GetClassificationsRequest,
) (*pb.GetClassificationsResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::GetClassifications")
	defer span.End()
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	img, err := rimage.DecodeImage(ctx, req.Image, req.MimeType)
	if err != nil {
		return nil, err
	}
	classifications, err := svc.Classifications(ctx, img, req.ClassifierName, int(req.N), req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	protoCs := make([]*pb.Classification, 0, len(classifications))
	for _, c := range classifications {
		cc := &pb.Classification{
			ClassName:  c.Label(),
			Confidence: c.Score(),
		}
		protoCs = append(protoCs, cc)
	}
	return &pb.GetClassificationsResponse{
		Classifications: protoCs,
	}, nil
}

func (server *subtypeServer) GetClassificationsFromCamera(
	ctx context.Context,
	req *pb.GetClassificationsFromCameraRequest,
) (*pb.GetClassificationsFromCameraResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::GetClassificationsFromCamera")
	defer span.End()
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	classifications, err := svc.ClassificationsFromCamera(ctx, req.CameraName, req.ClassifierName, int(req.N), req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	protoCs := make([]*pb.Classification, 0, len(classifications))
	for _, c := range classifications {
		cc := &pb.Classification{
			ClassName:  c.Label(),
			Confidence: c.Score(),
		}
		protoCs = append(protoCs, cc)
	}
	return &pb.GetClassificationsFromCameraResponse{
		Classifications: protoCs,
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
	names, err := svc.SegmenterNames(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetSegmenterNamesResponse{
		SegmenterNames: names,
	}, nil
}

func (server *subtypeServer) AddSegmenter(
	ctx context.Context,
	req *pb.AddSegmenterRequest,
) (*pb.AddSegmenterResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::AddSegmenter")
	defer span.End()
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	params := config.AttributeMap(req.SegmenterParameters.AsMap())
	cfg := VisModelConfig{
		Name:       req.SegmenterName,
		Type:       req.SegmenterModelType,
		Parameters: params,
	}
	err = svc.AddSegmenter(ctx, cfg, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.AddSegmenterResponse{}, nil
}

func (server *subtypeServer) RemoveSegmenter(
	ctx context.Context,
	req *pb.RemoveSegmenterRequest,
) (*pb.RemoveSegmenterResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::server::RemoveSegmenter")
	defer span.End()
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	err = svc.RemoveSegmenter(ctx, req.SegmenterName, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.RemoveSegmenterResponse{}, nil
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
	objects, err := svc.GetObjectPointClouds(ctx, req.CameraName, req.SegmenterName, req.Extra.AsMap())
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
				Geometries:     []*commonpb.Geometry{seg.Geometry.ToProtobuf()},
				ReferenceFrame: frame,
			},
		}
		protoSegs = append(protoSegs, ps)
	}
	return protoSegs, nil
}
