// Package camera contains a gRPC based camera service server.
package camera

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"

	"github.com/pkg/errors"
	"github.com/xfmoulet/qoi"
	"go.opencensus.io/trace"
	"google.golang.org/genproto/googleapis/api/httpbody"

	"go.viam.com/rdk/pointcloud"
	pb "go.viam.com/rdk/proto/api/component/camera/v1"
	"go.viam.com/rdk/rimage"
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
	ctx, span := trace.StartSpan(ctx, "camera-server::GetFrame")
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

	_, span3 := trace.StartSpan(ctx, "camera-server::GetFrame::Encode::"+req.MimeType)
	defer span3.End()
	var buf bytes.Buffer
	switch req.MimeType {
	case utils.MimeTypeRawRGBA:
		resp.MimeType = utils.MimeTypeRawRGBA
		imgCopy := image.NewRGBA(bounds)
		draw.Draw(imgCopy, bounds, img, bounds.Min, draw.Src)
		buf.Write(imgCopy.Pix)
	case utils.MimeTypeRawIWD:
		resp.MimeType = utils.MimeTypeRawIWD
		iwd, ok := img.(*rimage.ImageWithDepth)
		if !ok {
			return nil, errors.Errorf("want %s but don't have %T", utils.MimeTypeRawIWD, iwd)
		}
		err := iwd.RawBytesWrite(&buf)
		if err != nil {
			return nil, fmt.Errorf("error writing %s: %w", utils.MimeTypeRawIWD, err)
		}
	case utils.MimeTypeRawDepth:
		resp.MimeType = utils.MimeTypeRawDepth
		iwd, ok := img.(*rimage.ImageWithDepth)
		if !ok {
			return nil, utils.NewUnexpectedTypeError(iwd, img)
		}
		_, err := iwd.Depth.WriteTo(&buf)
		if err != nil {
			return nil, err
		}
	case utils.MimeTypeBoth:
		resp.MimeType = utils.MimeTypeBoth
		iwd, ok := img.(*rimage.ImageWithDepth)
		if !ok {
			return nil, errors.Errorf("want %s but don't have %T", utils.MimeTypeBoth, iwd)
		}
		if iwd.Color == nil || iwd.Depth == nil {
			return nil, errors.Errorf("for %s need depth and color info", utils.MimeTypeBoth)
		}
		if err := rimage.EncodeBoth(iwd, &buf); err != nil {
			return nil, err
		}
	case utils.MimeTypePNG:
		resp.MimeType = utils.MimeTypePNG
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
	case utils.MimeTypeJPEG:
		resp.MimeType = utils.MimeTypeJPEG
		if err := jpeg.Encode(&buf, img, nil); err != nil {
			return nil, err
		}
	case utils.MimeTypeQOI:
		resp.MimeType = utils.MimeTypeQOI
		if err := qoi.Encode(&buf, img); err != nil {
			return nil, err
		}
	default:
		return nil, errors.Errorf("do not know how to encode %q", req.MimeType)
	}
	resp.Image = buf.Bytes()
	return &resp, nil
}

// RenderFrame renders a frame from a camera of the underlying robot to an HTTP response. A specific MIME type
// can be requested but may not necessarily be the same one returned.
func (s *subtypeServer) RenderFrame(
	ctx context.Context,
	req *pb.RenderFrameRequest,
) (*httpbody.HttpBody, error) {
	ctx, span := trace.StartSpan(ctx, "camera-server::RenderFrame")
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
	ctx, span := trace.StartSpan(ctx, "camera-server::NextPointCloud")
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
	_, pcdSpan := trace.StartSpan(ctx, "camera-server::NextPointCloud::ToPCD")
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
