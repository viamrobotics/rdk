// Package camera contains a gRPC based camera service server.
package camera

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"google.golang.org/genproto/googleapis/api/httpbody"

	"go.viam.com/rdk/pointcloud"
	pb "go.viam.com/rdk/proto/api/component/camera/v1"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the CameraService from camera.proto.
type subtypeServer struct {
	pb.UnimplementedCameraServiceServer
	s subtype.Service
}

// NewServer constructs an camera gRPC service server.
func NewServer(s subtype.Service) pb.CameraServiceServer {
	return &subtypeServer{s: s}
}

// getCamera returns the camera specified, nil if not.
func (s *subtypeServer) getCamera(name string) (Camera, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no camera with name (%s)", name)
	}
	camera, ok := resource.(Camera)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a camera", name)
	}
	return camera, nil
}

// GetFrame returns a frame from a camera of the underlying robot. A specific MIME type
// can be requested but may not necessarily be the same one returned.
func (s *subtypeServer) GetFrame(
	ctx context.Context,
	req *pb.GetFrameRequest,
) (*pb.GetFrameResponse, error) {
	ctx, span := trace.StartSpan(ctx, "camera::server::GetFrame")
	defer span.End()
	camera, err := s.getCamera(req.Name)
	if err != nil {
		return nil, err
	}

	img, release, err := camera.Next(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if release != nil {
			release()
		}
	}()
	// choose the best/fastest representation
	if req.MimeType == "" || req.MimeType == utils.MimeTypeViamBest {
		switch img.(type) {
		case *rimage.ImageWithDepth:
			// TODO(DATA-237) remove this data type
			req.MimeType = utils.MimeTypeRawIWD
		default:
			req.MimeType = utils.MimeTypeRawRGBA
		}
	}

	bounds := img.Bounds()
	resp := pb.GetFrameResponse{
		MimeType: req.MimeType,
		WidthPx:  int64(bounds.Dx()),
		HeightPx: int64(bounds.Dy()),
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
	resp, err := s.GetFrame(ctx, (*pb.GetFrameRequest)(req))
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
	camera, err := s.getCamera(req.Name)
	if err != nil {
		return nil, err
	}
	proj, err := camera.GetProperties(ctx) // will be nil if no intrinsics
	if err != nil {
		return nil, err
	}
	intrinsics := proj.(*transform.PinholeCameraIntrinsics)
	err = intrinsics.CheckValid()
	if err != nil {
		return nil, err
	}

	camIntrinsics := &pb.IntrinsicParameters{
		WidthPx:   uint32(intrinsics.Width),
		HeightPx:  uint32(intrinsics.Height),
		FocalXPx:  intrinsics.Fx,
		FocalYPx:  intrinsics.Fy,
		CenterXPx: intrinsics.Ppx,
		CenterYPx: intrinsics.Ppy,
	}
	return &pb.GetPropertiesResponse{
		IntrinsicParameters: camIntrinsics,
	}, nil
}
