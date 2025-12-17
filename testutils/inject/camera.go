package inject

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/spatialmath"
)

// Camera is an injected camera.
type Camera struct {
	camera.Camera
	name                 resource.Name
	RTPPassthroughSource rtppassthrough.Source
	DoFunc               func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	ImagesFunc           func(
		ctx context.Context,
		filterSourceNames []string,
		extra map[string]interface{},
	) ([]camera.NamedImage, resource.ResponseMetadata, error)
	NextPointCloudFunc func(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error)
	ProjectorFunc      func(ctx context.Context) (transform.Projector, error)
	PropertiesFunc     func(ctx context.Context) (camera.Properties, error)
	CloseFunc          func(ctx context.Context) error
	GeometriesFunc     func(context.Context, map[string]interface{}) ([]spatialmath.Geometry, error)
}

// NewCamera returns a new injected camera.
func NewCamera(name string) *Camera {
	return &Camera{name: camera.Named(name)}
}

// Name returns the name of the resource.
func (c *Camera) Name() resource.Name {
	return c.name
}

// NextPointCloud calls the injected NextPointCloud or the real version.
func (c *Camera) NextPointCloud(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error) {
	if c.NextPointCloudFunc != nil {
		return c.NextPointCloudFunc(ctx, extra)
	}
	if c.Camera != nil {
		return c.Camera.NextPointCloud(ctx, extra)
	}
	return nil, errors.New("NextPointCloud unimplemented")
}

// Properties calls the injected Properties or the real version.
func (c *Camera) Properties(ctx context.Context) (camera.Properties, error) {
	if c.PropertiesFunc == nil {
		return c.Camera.Properties(ctx)
	}
	return c.PropertiesFunc(ctx)
}

// Images calls the injected Images or the real version.
func (c *Camera) Images(
	ctx context.Context,
	filterSourceNames []string,
	extra map[string]interface{},
) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	if c.ImagesFunc != nil {
		return c.ImagesFunc(ctx, filterSourceNames, extra)
	}

	if c.Camera != nil {
		return c.Camera.Images(ctx, filterSourceNames, extra)
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

// Geometries calls the injected Geometries or the real version.
func (c *Camera) Geometries(ctx context.Context, cmd map[string]interface{}) ([]spatialmath.Geometry, error) {
	if c.GeometriesFunc != nil {
		return c.GeometriesFunc(ctx, cmd)
	}
	return c.Camera.Geometries(ctx, cmd)
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
