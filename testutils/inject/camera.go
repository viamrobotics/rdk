package inject

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
)

// Camera is an injected camera.
type Camera struct {
	camera.Camera
	name                 resource.Name
	RTPPassthroughSource rtppassthrough.Source
	DoFunc               func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	ImageFunc            func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error)
	ImagesFunc           func(ctx context.Context) ([]camera.NamedImage, resource.ResponseMetadata, error)
	StreamFunc           func(
		ctx context.Context,
		errHandlers ...gostream.ErrorHandler,
	) (gostream.VideoStream, error)
	PointCloudFunc func(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error)
	ProjectorFunc  func(ctx context.Context) (transform.Projector, error)
	PropertiesFunc func(ctx context.Context) (camera.Properties, error)
	CloseFunc      func(ctx context.Context) error
}

// NewCamera returns a new injected camera.
func NewCamera(name string) *Camera {
	return &Camera{name: camera.Named(name)}
}

// Name returns the name of the resource.
func (c *Camera) Name() resource.Name {
	return c.name
}

// PointCloud calls the injected PointCloud or the real version.
func (c *Camera) PointCloud(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error) {
	if c.PointCloudFunc != nil {
		return c.PointCloudFunc(ctx, extra)
	}
	if c.Camera != nil {
		return c.Camera.PointCloud(ctx, extra)
	}
	return nil, errors.New("PointCloud unimplemented")
}

// Stream calls the injected Stream or the real version.
func (c *Camera) Stream(
	ctx context.Context,
	errHandlers ...gostream.ErrorHandler,
) (gostream.VideoStream, error) {
	if c.StreamFunc != nil {
		return c.StreamFunc(ctx, errHandlers...)
	}
	if c.Camera != nil {
		return c.Camera.Stream(ctx, errHandlers...)
	}
	return nil, errors.Wrap(ctx.Err(), "no stream function available")
}

// Image calls the injected Image or the real version.
func (c *Camera) Image(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
	if c.ImageFunc != nil {
		return c.ImageFunc(ctx, mimeType, extra)
	}
	if c.Camera != nil {
		return c.Camera.Image(ctx, mimeType, extra)
	}
	return nil, camera.ImageMetadata{}, errors.Wrap(ctx.Err(), "no Image function available")
}

// Properties calls the injected Properties or the real version.
func (c *Camera) Properties(ctx context.Context) (camera.Properties, error) {
	if c.PropertiesFunc == nil {
		return c.Camera.Properties(ctx)
	}
	return c.PropertiesFunc(ctx)
}

// Images calls the injected Images or the real version.
func (c *Camera) Images(ctx context.Context) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	if c.ImagesFunc != nil {
		return c.ImagesFunc(ctx)
	}

	if c.Camera != nil {
		return c.Camera.Images(ctx)
	}

	return nil, resource.ResponseMetadata{}, errors.New("Images unimplemented")
}

// Close calls the injected Close or the real version.
func (c *Camera) Close(ctx context.Context) error {
	if c.CloseFunc != nil {
		return c.CloseFunc(ctx)
	}
	if c.Camera != nil {
		return c.Camera.Close(ctx)
	}
	return nil
}

// DoCommand calls the injected DoCommand or the real version.
func (c *Camera) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if c.DoFunc != nil {
		return c.DoFunc(ctx, cmd)
	}
	return c.Camera.DoCommand(ctx, cmd)
}

// SubscribeRTP calls the injected RTPPassthroughSource or returns an error if unimplemented.
func (c *Camera) SubscribeRTP(
	ctx context.Context,
	bufferSize int,
	packetsCB rtppassthrough.PacketCallback,
) (rtppassthrough.Subscription, error) {
	if c.RTPPassthroughSource != nil {
		return c.RTPPassthroughSource.SubscribeRTP(ctx, bufferSize, packetsCB)
	}
	return rtppassthrough.NilSubscription, errors.New("SubscribeRTP unimplemented")
}

// Unsubscribe calls the injected RTPPassthroughSource or returns an error if unimplemented.
func (c *Camera) Unsubscribe(ctx context.Context, id rtppassthrough.SubscriptionID) error {
	if c.RTPPassthroughSource != nil {
		return c.RTPPassthroughSource.Unsubscribe(ctx, id)
	}
	return errors.New("Unsubscribe unimplemented")
}
