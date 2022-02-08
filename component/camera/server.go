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
	"google.golang.org/genproto/googleapis/api/httpbody"

	"go.viam.com/rdk/pointcloud"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/segmentation"
)

// subtypeServer implements the contract from camera.proto.
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
	req *pb.CameraServiceGetFrameRequest,
) (*pb.CameraServiceGetFrameResponse, error) {
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
		iwd, ok := img.(*rimage.ImageWithDepth)
		if ok && iwd.Depth != nil && iwd.Color != nil {
			req.MimeType = utils.MimeTypeRawIWD
		} else {
			req.MimeType = utils.MimeTypeRawRGBA
		}
	}

	bounds := img.Bounds()
	resp := pb.CameraServiceGetFrameResponse{
		MimeType: req.MimeType,
		WidthPx:  int64(bounds.Dx()),
		HeightPx: int64(bounds.Dy()),
	}

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
	default:
		return nil, errors.Errorf("do not know how to encode %q", req.MimeType)
	}
	resp.Frame = buf.Bytes()
	return &resp, nil
}

// RenderFrame renders a frame from a camera of the underlying robot to an HTTP response. A specific MIME type
// can be requested but may not necessarily be the same one returned.
func (s *subtypeServer) RenderFrame(
	ctx context.Context,
	req *pb.CameraServiceRenderFrameRequest,
) (*httpbody.HttpBody, error) {
	if req.MimeType == "" {
		req.MimeType = utils.MimeTypeJPEG // default rendering
	}
	resp, err := s.GetFrame(ctx, (*pb.CameraServiceGetFrameRequest)(req))
	if err != nil {
		return nil, err
	}

	return &httpbody.HttpBody{
		ContentType: resp.MimeType,
		Data:        resp.Frame,
	}, nil
}

// GetPointCloud returns a frame from a camera of the underlying robot. A specific MIME type
// can be requested but may not necessarily be the same one returned.
func (s *subtypeServer) GetPointCloud(
	ctx context.Context,
	req *pb.CameraServiceGetPointCloudRequest,
) (*pb.CameraServiceGetPointCloudResponse, error) {
	camera, err := s.getCamera(req.Name)
	if err != nil {
		return nil, err
	}

	pc, err := camera.NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = pc.ToPCD(&buf)
	if err != nil {
		return nil, err
	}

	return &pb.CameraServiceGetPointCloudResponse{
		MimeType: utils.MimeTypePCD,
		Frame:    buf.Bytes(),
	}, nil
}

// GetObjectPointClouds returns an array of objects from the frame from a camera of the underlying robot. A specific MIME type
// can be requested but may not necessarily be the same one returned. Also returns a Vector3 array of the center points of each object.
func (s *subtypeServer) GetObjectPointClouds(
	ctx context.Context,
	req *pb.CameraServiceGetObjectPointCloudsRequest,
) (*pb.CameraServiceGetObjectPointCloudsResponse, error) {
	camera, err := s.getCamera(req.Name)
	if err != nil {
		return nil, err
	}

	pc, err := camera.NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}
	config := segmentation.ObjectConfig{
		MinPtsInPlane:      int(req.MinPointsInPlane),
		MinPtsInSegment:    int(req.MinPointsInSegment),
		ClusteringRadiusMm: req.ClusteringRadiusMm,
	}
	segments, err := segmentation.NewObjectSegmentation(ctx, pc, config)
	if err != nil {
		return nil, err
	}
	protoSegments, err := segmentsToProto(segments)
	if err != nil {
		return nil, err
	}

	return &pb.CameraServiceGetObjectPointCloudsResponse{
		MimeType: utils.MimeTypePCD,
		Objects:  protoSegments,
	}, nil
}

func segmentsToProto(segs *segmentation.ObjectSegmentation) ([]*pb.PointCloudObject, error) {
	protoSegs := make([]*pb.PointCloudObject, 0, segs.N())
	for _, seg := range segs.Objects {
		var buf bytes.Buffer
		err := seg.ToPCD(&buf)
		if err != nil {
			return nil, err
		}
		ps := &pb.PointCloudObject{
			Frame:               buf.Bytes(),
			CenterCoordinatesMm: pointToProto(seg.Center),
			BoundingBoxMm:       boxToProto(seg.BoundingBox),
		}
		protoSegs = append(protoSegs, ps)
	}
	return protoSegs, nil
}

func pointToProto(p pointcloud.Vec3) *commonpb.Vector3 {
	return &commonpb.Vector3{
		X: p.X,
		Y: p.Y,
		Z: p.Z,
	}
}

func boxToProto(b pointcloud.BoxGeometry) *commonpb.BoxGeometry {
	return &commonpb.BoxGeometry{
		WidthMm:  b.WidthMm,
		LengthMm: b.LengthMm,
		DepthMm:  b.DepthMm,
	}
}
