package camera

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"sync"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"go.opencensus.io/trace"
	pb "go.viam.com/api/component/camera/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

// client implements CameraServiceClient.
type client struct {
	mu                      sync.Mutex
	name                    string
	conn                    rpc.ClientConn
	client                  pb.CameraServiceClient
	logger                  golog.Logger
	activeBackgroundWorkers sync.WaitGroup
	cancelCtx               context.Context
	cancel                  func()
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Camera {
	cancelCtx, cancel := context.WithCancel(context.Background())
	c := pb.NewCameraServiceClient(conn)
	return &client{
		name:      name,
		conn:      conn,
		client:    c,
		logger:    logger,
		cancelCtx: cancelCtx,
		cancel:    cancel,
	}
}

func (c *client) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::client::Read")
	defer span.End()
	mimeType := gostream.MIMETypeHint(ctx, "")
	expectedType, _ := utils.CheckLazyMIMEType(mimeType)
	resp, err := c.client.GetImage(ctx, &pb.GetImageRequest{
		Name:     c.name,
		MimeType: expectedType,
	})
	if err != nil {
		return nil, nil, err
	}

	if resp.MimeType != expectedType {
		c.logger.Debugw("got different MIME type than what was asked for", "sent", expectedType, "received", resp.MimeType)
	} else {
		resp.MimeType = mimeType
	}

	resp.MimeType = utils.WithLazyMIMEType(resp.MimeType)
	img, err := rimage.DecodeImage(ctx, resp.Image, resp.MimeType)
	if err != nil {
		return nil, nil, err
	}
	return img, func() {}, nil
}

func (c *client) Stream(
	ctx context.Context,
	errHandlers ...gostream.ErrorHandler,
) (gostream.VideoStream, error) {
	cancelCtxWithMIME := gostream.WithMIMETypeHint(c.cancelCtx, gostream.MIMETypeHint(ctx, ""))
	streamCtx, stream, frameCh := gostream.NewMediaStreamForChannel[image.Image](cancelCtxWithMIME)

	c.mu.Lock()
	if err := c.cancelCtx.Err(); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	c.activeBackgroundWorkers.Add(1)
	c.mu.Unlock()

	goutils.PanicCapturingGo(func() {
		defer c.activeBackgroundWorkers.Done()
		defer close(frameCh)

		for {
			if streamCtx.Err() != nil {
				return
			}

			frame, release, err := c.Read(streamCtx)
			if err != nil {
				for _, handler := range errHandlers {
					handler(streamCtx, err)
				}
			}

			select {
			case <-streamCtx.Done():
				return
			case frameCh <- gostream.MediaReleasePairWithError[image.Image]{
				Media:   frame,
				Release: release,
				Err:     err,
			}:
			}
		}
	})

	return stream, nil
}

func (c *client) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "camera::client::NextPointCloud")
	defer span.End()

	ctx, getPcdSpan := trace.StartSpan(ctx, "camera::client::NextPointCloud::GetPointCloud")
	resp, err := c.client.GetPointCloud(ctx, &pb.GetPointCloudRequest{
		Name:     c.name,
		MimeType: utils.MimeTypePCD,
	})
	getPcdSpan.End()
	if err != nil {
		return nil, err
	}

	if resp.MimeType != utils.MimeTypePCD {
		return nil, fmt.Errorf("unknown pc mime type %s", resp.MimeType)
	}

	return func() (pointcloud.PointCloud, error) {
		_, span := trace.StartSpan(ctx, "camera::client::NextPointCloud::ReadPCD")
		defer span.End()

		return pointcloud.ReadPCD(bytes.NewReader(resp.PointCloud))
	}()
}

func (c *client) Projector(ctx context.Context) (transform.Projector, error) {
	var proj transform.Projector
	props, err := c.Properties(ctx)
	if err != nil {
		return nil, err
	}
	intrinsics := props.IntrinsicParams
	err = intrinsics.CheckValid()
	if err != nil {
		return nil, err
	}
	proj = intrinsics
	return proj, nil
}

func (c *client) Properties(ctx context.Context) (Properties, error) {
	result := Properties{}
	resp, err := c.client.GetProperties(ctx, &pb.GetPropertiesRequest{
		Name: c.name,
	})
	if err != nil {
		return Properties{}, err
	}
	if intrinsics := resp.IntrinsicParameters; intrinsics != nil {
		result.IntrinsicParams = &transform.PinholeCameraIntrinsics{
			Width:  int(intrinsics.WidthPx),
			Height: int(intrinsics.HeightPx),
			Fx:     intrinsics.FocalXPx,
			Fy:     intrinsics.FocalYPx,
			Ppx:    intrinsics.CenterXPx,
			Ppy:    intrinsics.CenterYPx,
		}
	}
	result.SupportsPCD = resp.SupportsPcd
	// if no distortion model present, return result with no model
	if resp.DistortionParameters == nil {
		return result, nil
	}
	// switch distortion model based on model name
	model := transform.DistortionType(resp.DistortionParameters.Model)
	distorter, err := transform.NewDistorter(model, resp.DistortionParameters.Parameters)
	if err != nil {
		return Properties{}, err
	}
	result.DistortionParams = distorter
	return result, nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

func (c *client) Close(ctx context.Context) error {
	c.mu.Lock()
	c.cancel()
	c.mu.Unlock()
	c.activeBackgroundWorkers.Wait()
	return nil
}
