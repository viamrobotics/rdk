package vision

import (
	"context"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	pb "go.viam.com/rdk/proto/api/service/vision/v1"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the Object Detection Service.
type subtypeServer struct {
	pb.UnimplementedVisionServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a object detection gRPC service server.
func NewServer(s subtype.Service) pb.VisionServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service() (Service, error) {
	resource := server.subtypeSvc.Resource(Name.String())
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("objectdetection.Service", resource)
	}
	return svc, nil
}

func (server *subtypeServer) DetectorNames(
	ctx context.Context,
	req *pb.DetectorNamesRequest,
) (*pb.DetectorNamesResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::objectdetection::server::DetectorNames")
	defer span.End()
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	names, err := svc.DetectorNames(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.DetectorNamesResponse{
		DetectorNames: names,
	}, nil
}

func (server *subtypeServer) AddDetector(
	ctx context.Context,
	req *pb.AddDetectorRequest,
) (*pb.AddDetectorResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::objectdetection::server::AddDetector")
	defer span.End()
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	params := config.AttributeMap(req.DetectorParameters.AsMap())
	cfg := Config{
		Name:       req.DetectorName,
		Type:       req.DetectorModelType,
		Parameters: params,
	}
	success, err := svc.AddDetector(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &pb.AddDetectorResponse{
		Success: success,
	}, nil
}

func (server *subtypeServer) Detect(
	ctx context.Context,
	req *pb.DetectRequest,
) (*pb.DetectResponse, error) {
	ctx, span := trace.StartSpan(ctx, "service::objectdetection::server::Detect")
	defer span.End()
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	detections, err := svc.Detect(ctx, req.CameraName, req.DetectorName)
	if err != nil {
		return nil, err
	}
	protoDets := make([]*pb.Detection, 0, len(detections))
	for _, det := range detections {
		box := det.BoundingBox()
		if box == nil {
			return nil, errors.New("detection has no bounding box, must return a bounding box")
		}
		d := &pb.Detection{
			XMin:       int64(box.Min.X),
			YMin:       int64(box.Min.Y),
			XMax:       int64(box.Max.X),
			YMax:       int64(box.Max.Y),
			Confidence: det.Score(),
			ClassName:  det.Label(),
		}
		protoDets = append(protoDets, d)
	}
	return &pb.DetectResponse{
		Detections: protoDets,
	}, nil
}
