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

// subtypeServer implements the contract from camera.proto
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

// Frame returns a frame from a camera of the underlying robot. A specific MIME type
// can be requested but may not necessarily be the same one returned.
func (s *subtypeServer) Frame(
	ctx context.Context,
	req *pb.CameraServiceFrameRequest,
) (*pb.CameraServiceFrameResponse, error) {
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
	if req.MimeType == utils.MimeTypeViamBest {
		iwd, ok := img.(*rimage.ImageWithDepth)
		if ok && iwd.Depth != nil && iwd.Color != nil {
			req.MimeType = utils.MimeTypeRawIWD
		} else {
			req.MimeType = utils.MimeTypeRawRGBA
		}
	}

	bounds := img.Bounds()
	resp := pb.CameraServiceFrameResponse{
		MimeType: req.MimeType,
		DimX:     int64(bounds.Dx()),
		DimY:     int64(bounds.Dy()),
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
	case utils.MimeTypeJPEG:
		resp.MimeType = utils.MimeTypeJPEG
		if err := jpeg.Encode(&buf, img, nil); err != nil {
			return nil, err
		}
	case "", utils.MimeTypePNG:
		resp.MimeType = utils.MimeTypePNG
		if err := png.Encode(&buf, img); err != nil {
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
	resp, err := s.Frame(ctx, (*pb.CameraServiceFrameRequest)(req))
	if err != nil {
		return nil, err
	}

	return &httpbody.HttpBody{
		ContentType: resp.MimeType,
		Data:        resp.Frame,
	}, nil
}

// PointCloud returns a frame from a camera of the underlying robot. A specific MIME type
// can be requested but may not necessarily be the same one returned.
func (s *subtypeServer) PointCloud(
	ctx context.Context,
	req *pb.CameraServicePointCloudRequest,
) (*pb.CameraServicePointCloudResponse, error) {
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

	return &pb.CameraServicePointCloudResponse{
		MimeType: utils.MimeTypePCD,
		Frame:    buf.Bytes(),
	}, nil
}

// ObjectPointClouds returns an array of objects from the frame from a camera of the underlying robot. A specific MIME type
// can be requested but may not necessarily be the same one returned. Also returns a Vector3 array of the center points of each object.
func (s *subtypeServer) ObjectPointClouds(
	ctx context.Context,
	req *pb.CameraServiceObjectPointCloudsRequest,
) (*pb.CameraServiceObjectPointCloudsResponse, error) {
	camera, err := s.getCamera(req.Name)
	if err != nil {
		return nil, err
	}

	pc, err := camera.NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}
	config := segmentation.ObjectConfig{
		MinPtsInPlane:    int(req.MinPointsInPlane),
		MinPtsInSegment:  int(req.MinPointsInSegment),
		ClusteringRadius: req.ClusteringRadius,
	}
	segments, err := segmentation.NewObjectSegmentation(ctx, pc, config)
	if err != nil {
		return nil, err
	}

	frames := make([][]byte, segments.N())
	centers := make([]pointcloud.Vec3, segments.N())
	boundingBoxes := make([]pointcloud.BoxGeometry, segments.N())
	for i, seg := range segments.Objects {
		var buf bytes.Buffer
		err := seg.ToPCD(&buf)
		if err != nil {
			return nil, err
		}
		frames[i] = buf.Bytes()
		centers[i] = seg.Center
		boundingBoxes[i] = seg.BoundingBox
	}

	return &pb.CameraServiceObjectPointCloudsResponse{
		MimeType:      utils.MimeTypePCD,
		Frames:        frames,
		Centers:       pointsToProto(centers),
		BoundingBoxes: boxesToProto(boundingBoxes),
	}, nil
}

func pointToProto(p pointcloud.Vec3) *commonpb.Vector3 {
	return &commonpb.Vector3{
		X: p.X,
		Y: p.Y,
		Z: p.Z,
	}
}

func pointsToProto(vs []pointcloud.Vec3) []*commonpb.Vector3 {
	pvs := make([]*commonpb.Vector3, 0, len(vs))
	for _, v := range vs {
		pvs = append(pvs, pointToProto(v))
	}
	return pvs
}

func boxToProto(b pointcloud.BoxGeometry) *commonpb.BoxGeometry {
	return &commonpb.BoxGeometry{
		Width:  b.Width,
		Length: b.Length,
		Depth:  b.Depth,
	}
}

func boxesToProto(bs []pointcloud.BoxGeometry) []*commonpb.BoxGeometry {
	pbs := make([]*commonpb.BoxGeometry, 0, len(bs))
	for _, v := range bs {
		pbs = append(pbs, boxToProto(v))
	}
	return pbs
}
