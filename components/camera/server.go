package camera

import (
	"bytes"
	"context"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	pb "go.viam.com/api/component/camera/v1"
	"google.golang.org/genproto/googleapis/api/httpbody"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the CameraService from camera.proto.
type subtypeServer struct {
	pb.UnimplementedCameraServiceServer
	s        subtype.Service
	imgTypes map[string]ImageType
	logger   golog.Logger
}

// NewServer constructs an camera gRPC service server.
func NewServer(s subtype.Service) pb.CameraServiceServer {
	logger := golog.NewLogger("camserver")
	imgTypes := make(map[string]ImageType)
	return &subtypeServer{s: s, logger: logger, imgTypes: imgTypes}
}

// getCamera returns the camera specified, nil if not.
func (s *subtypeServer) getCamera(name string) (Camera, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no camera with name (%s)", name)
	}
	cam, ok := resource.(Camera)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a camera", name)
	}
	return cam, nil
}

// GetImage returns an image from a camera of the underlying robot. If a specific MIME type
// is requested and is not available, an error is returned.
func (s *subtypeServer) GetImage(
	ctx context.Context,
	req *pb.GetImageRequest,
) (*pb.GetImageResponse, error) {
	ctx, span := trace.StartSpan(ctx, "camera::server::GetImage")
	defer span.End()
	cam, err := s.getCamera(req.Name)
	if err != nil {
		return nil, err
	}

	// Determine the mimeType we should try to use based on camera properties
	if req.MimeType == "" {
		if _, ok := s.imgTypes[req.Name]; !ok {
			props, err := cam.Properties(ctx)
			if err != nil {
				s.logger.Warnf("camera properties not found for %s, assuming color images: %v", req.Name, err)
				s.imgTypes[req.Name] = ColorStream
			} else {
				s.imgTypes[req.Name] = props.ImageType
			}
		}
		switch s.imgTypes[req.Name] {
		case ColorStream, UnspecifiedStream:
			req.MimeType = utils.MimeTypeJPEG
		case DepthStream:
			req.MimeType = utils.MimeTypeRawDepth
		default:
			req.MimeType = utils.MimeTypeJPEG
		}
	}

	req.MimeType = utils.WithLazyMIMEType(req.MimeType)
	img, release, err := ReadImage(gostream.WithMIMETypeHint(ctx, req.MimeType), cam)
	if err != nil {
		return nil, err
	}
	defer func() {
		if release != nil {
			release()
		}
	}()

	actualMIME, _ := utils.CheckLazyMIMEType(req.MimeType)
	resp := pb.GetImageResponse{
		MimeType: actualMIME,
	}
	outBytes, err := rimage.EncodeImage(ctx, img, req.MimeType)
	if err != nil {
		return nil, err
	}
	resp.Image = outBytes
	return &resp, nil
}

// RenderFrame renders a frame from a camera of the underlying robot to an HTTP response. A specific MIME type
// can be requested but may not necessarily be the same one returned.
func (s *subtypeServer) RenderFrame(
	ctx context.Context,
	req *pb.RenderFrameRequest,
) (*httpbody.HttpBody, error) {
	ctx, span := trace.StartSpan(ctx, "camera::server::RenderFrame")
	defer span.End()
	if req.MimeType == "" {
		req.MimeType = utils.MimeTypeJPEG // default rendering
	}
	resp, err := s.GetImage(ctx, (*pb.GetImageRequest)(req))
	if err != nil {
		return nil, err
	}

	return &httpbody.HttpBody{
		ContentType: resp.MimeType,
		Data:        resp.Image,
	}, nil
}

// GetPointCloud returns a frame from a camera of the underlying robot. A specific MIME type
// can be requested but may not necessarily be the same one returned.
func (s *subtypeServer) GetPointCloud(
	ctx context.Context,
	req *pb.GetPointCloudRequest,
) (*pb.GetPointCloudResponse, error) {
	ctx, span := trace.StartSpan(ctx, "camera::server::GetPointCloud")
	defer span.End()
	camera, err := s.getCamera(req.Name)
	if err != nil {
		return nil, err
	}

	pc, err := camera.NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Grow(200 + (pc.Size() * 4 * 4)) // 4 numbers per point, each 4 bytes
	_, pcdSpan := trace.StartSpan(ctx, "camera::server::NextPointCloud::ToPCD")
	err = pointcloud.ToPCD(pc, &buf, pointcloud.PCDBinary)
	pcdSpan.End()
	if err != nil {
		return nil, err
	}

	return &pb.GetPointCloudResponse{
		MimeType:   utils.MimeTypePCD,
		PointCloud: buf.Bytes(),
	}, nil
}

func (s *subtypeServer) GetProperties(
	ctx context.Context,
	req *pb.GetPropertiesRequest,
) (*pb.GetPropertiesResponse, error) {
	result := &pb.GetPropertiesResponse{}
	camera, err := s.getCamera(req.Name)
	if err != nil {
		return nil, err
	}
	props, err := camera.Properties(ctx)
	if err != nil {
		return nil, err
	}
	intrinsics := props.IntrinsicParams
	if intrinsics != nil {
		result.IntrinsicParameters = &pb.IntrinsicParameters{
			WidthPx:   uint32(intrinsics.Width),
			HeightPx:  uint32(intrinsics.Height),
			FocalXPx:  intrinsics.Fx,
			FocalYPx:  intrinsics.Fy,
			CenterXPx: intrinsics.Ppx,
			CenterYPx: intrinsics.Ppy,
		}
	}
	result.SupportsPcd = props.SupportsPCD
	if props.DistortionParams != nil {
		result.DistortionParameters = &pb.DistortionParameters{
			Model:      string(props.DistortionParams.ModelType()),
			Parameters: props.DistortionParams.Parameters(),
		}
	}
	return result, nil
}
