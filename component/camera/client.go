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
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/pointcloud"
	pb "go.viam.com/rdk/proto/api/component/camera/v1"
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
	mimeType := gostream.MIMETypeHint(ctx, utils.MimeTypeRawRGBALazy)
	actualType, _ := utils.CheckLazyMIMEType(mimeType)
	resp, err := c.client.GetFrame(ctx, &pb.GetFrameRequest{
		Name:     c.name,
		MimeType: mimeType,
	})
	if err != nil {
		return nil, nil, err
	}
	if actualType != resp.MimeType {
		c.logger.Debugw("got different MIME type than what was asked for", "sent", actualType, "received", resp.MimeType)
	} else {
		resp.MimeType = mimeType
	}
	img, err := rimage.DecodeImage(ctx, resp.Image, resp.MimeType, int(resp.WidthPx), int(resp.HeightPx))
	if err != nil {
		return nil, nil, err
	}
	return img, func() {}, nil
}

func (c *client) Stream(
	ctx context.Context,
	errHandlers ...gostream.ErrorHandler,
) (gostream.VideoStream, error) {
	cancelCtxWithMIME := gostream.WithMIMETypeHint(c.cancelCtx, gostream.MIMETypeHint(ctx, utils.MimeTypeRawRGBALazy))
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

func (c *client) Projector(ctx context.Context) (rimage.Projector, error) {
	var proj rimage.Projector
	props, err := c.GetProperties(ctx)
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

func (c *client) GetProperties(ctx context.Context) (Properties, error) {
	result := Properties{}
	resp, err := c.client.GetProperties(ctx, &pb.GetPropertiesRequest{
		Name: c.name,
	})
	if err != nil {
		return Properties{}, err
	}
	result.IntrinsicParams = &transform.PinholeCameraIntrinsics{
		Width:      int(resp.IntrinsicParameters.WidthPx),
		Height:     int(resp.IntrinsicParameters.HeightPx),
		Fx:         resp.IntrinsicParameters.FocalXPx,
		Fy:         resp.IntrinsicParameters.FocalYPx,
		Ppx:        resp.IntrinsicParameters.CenterXPx,
		Ppy:        resp.IntrinsicParameters.CenterYPx,
		Distortion: transform.DistortionModel{},
	}
	result.SupportsPCD = resp.SupportsPcd
	return result, nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

func (c *client) Close(ctx context.Context) error {
	c.mu.Lock()
	c.cancel()
	c.mu.Unlock()
	c.activeBackgroundWorkers.Wait()
	return nil
}
