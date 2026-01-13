package camera

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/camera/v1"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/trace"
	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/grpc/metadata"

	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

// serviceServer implements the CameraService from camera.proto.
type serviceServer struct {
	pb.UnimplementedCameraServiceServer
	coll resource.APIResourceGetter[Camera]

	imgTypesMu sync.RWMutex
	imgTypes   map[string]ImageType
	logger     logging.Logger

	// lastImageDeprecationLogNanos stores Unix nanoseconds of last Image deprecation log (atomic)
	lastImageDeprecationLogNanos atomic.Int64
}

// NewRPCServiceServer constructs an camera gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Camera], logger logging.Logger) interface{} {
	imgTypes := make(map[string]ImageType)
	return &serviceServer{
		coll:     coll,
		logger:   logger.Sublogger("cam_server"),
		imgTypes: imgTypes,
	}
}

// GetImage returns an image from a camera of the underlying robot. If a specific MIME type
// is requested and is not available, an error is returned.
func (s *serviceServer) GetImage(
	ctx context.Context,
	req *pb.GetImageRequest,
) (*pb.GetImageResponse, error) {
	now := time.Now()
	lastLog := s.lastImageDeprecationLogNanos.Load()
	if now.UnixNano()-lastLog >= int64(10*time.Minute) {
		// Try to update the timestamp; if another goroutine updated it first, that's fine.
		if s.lastImageDeprecationLogNanos.CompareAndSwap(lastLog, now.UnixNano()) {
			peerInfo := rpc.PeerConnectionInfoFromContext(ctx)
			moduleName := grpc.GetModuleName(ctx)
			md, _ := metadata.FromIncomingContext(ctx)

			s.logger.Warnw("camera server: GetImage is deprecated; please use GetImages instead",
				"camera_name", req.Name,
				"peer_remote_addr", peerInfo.RemoteAddress,
				"module_name", moduleName,
				"grpc_metadata", md,
			)
		}
	}
	ctx, span := trace.StartSpan(ctx, "camera::server::GetImage")
	defer span.End()
	cam, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	// Determine the mimeType we should try to use based on camera properties
	if req.MimeType == "" {
		s.imgTypesMu.RLock()
		imgType, ok := s.imgTypes[req.Name]
		s.imgTypesMu.RUnlock()
		if !ok {
			props, err := cam.Properties(ctx)
			if err != nil {
				s.logger.CWarnf(ctx, "camera properties not found for %s, assuming color images: %v", req.Name, err)
				imgType = ColorStream
			} else {
				imgType = props.ImageType
			}
			s.imgTypesMu.Lock()
			s.imgTypes[req.Name] = imgType
			s.imgTypesMu.Unlock()
		}
		switch imgType {
		case ColorStream, UnspecifiedStream:
			req.MimeType = utils.MimeTypeJPEG
		case DepthStream:
			req.MimeType = utils.MimeTypeRawDepth
		default:
			req.MimeType = utils.MimeTypeJPEG
		}
	}
	req.MimeType = utils.WithLazyMIMEType(req.MimeType)

	resBytes, resMetadata, err := cam.Image(ctx, req.MimeType, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	if len(resBytes) == 0 {
		return nil, fmt.Errorf("received empty bytes from Image method of %s", req.Name)
	}
	actualMIME, _ := utils.CheckLazyMIMEType(resMetadata.MimeType)
	return &pb.GetImageResponse{MimeType: actualMIME, Image: resBytes}, nil
}

// GetImages returns a list of images and metadata from a camera of the underlying robot.
func (s *serviceServer) GetImages(
	ctx context.Context,
	req *pb.GetImagesRequest,
) (*pb.GetImagesResponse, error) {
	ctx, span := trace.StartSpan(ctx, "camera::server::GetImages")
	defer span.End()
	cam, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, errors.Wrap(err, "camera server GetImages had an error getting the camera component")
	}

	if len(req.FilterSourceNames) > 1 {
		seen := make(map[string]bool)
		for _, sourceName := range req.FilterSourceNames {
			if seen[sourceName] {
				return nil, fmt.Errorf("duplicate source name in filter: %s", sourceName)
			}
			seen[sourceName] = true
		}
	}

	// request the images, and then check to see what the underlying type is to determine
	// what to encode as. If it's color, just encode as JPEG.
	imgs, metadata, err := cam.Images(ctx, req.FilterSourceNames, req.Extra.AsMap())
	if err != nil {
		return nil, errors.Wrap(err, "camera server GetImages could not call Images on the camera")
	}
	imagesMessage := make([]*pb.Image, 0, len(imgs))
	for _, img := range imgs {
		imgBytes, err := img.Bytes(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "camera server GetImages could not get the image bytes")
		}
		imgMes := &pb.Image{
			SourceName:  img.SourceName,
			MimeType:    img.MimeType(),
			Image:       imgBytes,
			Annotations: img.Annotations.ToProto(),
		}
		imagesMessage = append(imagesMessage, imgMes)
	}
	// right now the only metadata is timestamp
	resp := &pb.GetImagesResponse{
		Images:           imagesMessage,
		ResponseMetadata: metadata.AsProto(),
	}

	return resp, nil
}

// RenderFrame renders a frame from a camera of the underlying robot to an HTTP response. A specific MIME type
// can be requested but may not necessarily be the same one returned.
// Deprecated: Use GetImages instead.
func (s *serviceServer) RenderFrame(
	ctx context.Context,
	req *pb.RenderFrameRequest,
) (*httpbody.HttpBody, error) {
	s.logger.CWarn(ctx, "RenderFrame is deprecated; please use GetImages instead")
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
func (s *serviceServer) GetPointCloud(
	ctx context.Context,
	req *pb.GetPointCloudRequest,
) (*pb.GetPointCloudResponse, error) {
	ctx, span := trace.StartSpan(ctx, "camera::server::GetPointCloud")
	defer span.End()
	camera, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	if camClient, ok := camera.(*client); ok {
		return camClient.client.GetPointCloud(ctx, req)
	}

	pc, err := camera.NextPointCloud(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}

	bytes, err := pointcloud.ToBytes(pc)
	if err != nil {
		return nil, err
	}

	return &pb.GetPointCloudResponse{
		MimeType:   utils.MimeTypePCD,
		PointCloud: bytes,
	}, nil
}

func (s *serviceServer) GetProperties(
	ctx context.Context,
	req *pb.GetPropertiesRequest,
) (*pb.GetPropertiesResponse, error) {
	result := &pb.GetPropertiesResponse{}
	camera, err := s.coll.Resource(req.Name)
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

	if props.FrameRate != 0 {
		result.FrameRate = &props.FrameRate
	}

	result.MimeTypes = props.MimeTypes
	return result, nil
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	camera, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, camera, req)
}

func (s *serviceServer) GetGeometries(ctx context.Context, req *commonpb.GetGeometriesRequest) (*commonpb.GetGeometriesResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	geometries, err := res.Geometries(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &commonpb.GetGeometriesResponse{Geometries: referenceframe.NewGeometriesToProto(geometries)}, nil
}
