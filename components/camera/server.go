package camera

import (
	"bytes"
	"context"
	"image"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"github.com/viamrobotics/gostream"
	"go.opencensus.io/trace"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/camera/v1"
	"google.golang.org/genproto/googleapis/api/httpbody"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

// serviceServer implements the CameraService from camera.proto.
type serviceServer struct {
	pb.UnimplementedCameraServiceServer
	coll     resource.APIResourceCollection[Camera]
	imgTypes map[string]ImageType
	logger   golog.Logger
}

// NewRPCServiceServer constructs an camera gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceCollection[Camera]) interface{} {
	logger := golog.NewLogger("camserver")
	imgTypes := make(map[string]ImageType)
	return &serviceServer{coll: coll, logger: logger, imgTypes: imgTypes}
}

// GetImage returns an image from a camera of the underlying robot. If a specific MIME type
// is requested and is not available, an error is returned.
func (s *serviceServer) GetImage(
	ctx context.Context,
	req *pb.GetImageRequest,
) (*pb.GetImageResponse, error) {
	ctx, span := trace.StartSpan(ctx, "camera::server::GetImage")
	defer span.End()
	cam, err := s.coll.Resource(req.Name)
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

	// Add 'fromDataManagement' to context to avoid threading extra through gostream API.
	if req.Extra.AsMap()[data.FromDMString] == true {
		ctx = context.WithValue(ctx, data.FromDMContextKey{}, true)
	}

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
	// request the images, and then check to see what the underlying type is to determine
	// what to encode as. If it's color, just encode as JPEG.
	imgs, metadata, err := cam.Images(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "camera server GetImages could not call Images on the camera")
	}
	imagesMessage := make([]*pb.Image, 0, len(imgs))
	for _, img := range imgs {
		format, outBytes, err := encodeImageFromUnderlyingType(ctx, img.Image)
		if err != nil {
			return nil, errors.Wrap(err, "camera server GetImages could not encode the images")
		}
		imgMes := &pb.Image{
			SourceName: img.SourceName,
			Format:     format,
			Image:      outBytes,
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

func encodeImageFromUnderlyingType(ctx context.Context, img image.Image) (pb.Format, []byte, error) {
	switch v := img.(type) {
	case *rimage.LazyEncodedImage:
		format := pb.Format_FORMAT_UNSPECIFIED
		switch v.MIMEType() {
		case utils.MimeTypeRawDepth:
			format = pb.Format_FORMAT_RAW_DEPTH
		case utils.MimeTypeRawRGBA:
			format = pb.Format_FORMAT_RAW_RGBA
		case utils.MimeTypeJPEG:
			format = pb.Format_FORMAT_JPEG
		case utils.MimeTypePNG:
			format = pb.Format_FORMAT_PNG
		default:
		}
		return format, v.RawData(), nil
	case *rimage.DepthMap:
		format := pb.Format_FORMAT_RAW_DEPTH
		outBytes, err := rimage.EncodeImage(ctx, v, utils.MimeTypeRawDepth)
		if err != nil {
			return pb.Format_FORMAT_UNSPECIFIED, nil, err
		}
		return format, outBytes, nil
	case *image.Gray16:
		format := pb.Format_FORMAT_PNG
		outBytes, err := rimage.EncodeImage(ctx, v, utils.MimeTypePNG)
		if err != nil {
			return pb.Format_FORMAT_UNSPECIFIED, nil, err
		}
		return format, outBytes, nil
	default:
		format := pb.Format_FORMAT_JPEG
		outBytes, err := rimage.EncodeImage(ctx, v, utils.MimeTypeJPEG)
		if err != nil {
			return pb.Format_FORMAT_UNSPECIFIED, nil, err
		}
		return format, outBytes, nil
	}
}

// RenderFrame renders a frame from a camera of the underlying robot to an HTTP response. A specific MIME type
// can be requested but may not necessarily be the same one returned.
func (s *serviceServer) RenderFrame(
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
