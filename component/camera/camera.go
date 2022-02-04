// Package camera defines a frame capturing device.
package camera

import (
	"context"
	"image"
	"sync"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rlog"
)

// SubtypeName is a constant that identifies the camera resource subtype string.
const SubtypeName = resource.SubtypeName("camera")

// Subtype is a constant that identifies the camera resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named cameras's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// A Camera represents anything that can capture frames.
type Camera interface {
	gostream.ImageSource
	NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error)
}

// WithProjector is a camera with the capability to project images to 3D.
type WithProjector interface {
	Camera
	rimage.Projector
	GetProjector() rimage.Projector
}

// New creates a Camera either with or without a projector, depending on if the camera config has the parameters,
// or if it has a parent Camera with camera parameters that it should copy. parentSource and attrs can be nil.
func New(imgSrc gostream.ImageSource, attrs *AttrConfig, parentSource Camera) (Camera, error) {
	if imgSrc == nil {
		return nil, errors.New("cannot have a nil image source")
	}
	// if the camera parameters are specified in the config, those get priority.
	if attrs != nil && attrs.CameraParameters != nil {
		return &imageSourceWithProjector{imgSrc, attrs.CameraParameters}, nil
	}
	// inherit camera parameters from source camera if possible. if not, create a camera without projector.
	if reconfigCam, ok := parentSource.(*reconfigurableCamera); ok {
		if c, ok := reconfigCam.ProxyFor().(WithProjector); ok {
			return &imageSourceWithProjector{imgSrc, c.GetProjector()}, nil
		}
	}
	if camera, ok := parentSource.(WithProjector); ok {
		return &imageSourceWithProjector{imgSrc, camera.GetProjector()}, nil
	}
	return &imageSource{imgSrc}, nil
}

// ImageSource implements a Camera with a gostream.ImageSource.
type imageSource struct {
	gostream.ImageSource
}

// Close closes the underlying ImageSource.
func (is *imageSource) Close(ctx context.Context) error {
	return utils.TryClose(ctx, is.ImageSource)
}

// NextPointCloud returns the next PointCloud from the camera, or will error if not supported.
func (is *imageSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	if c, ok := is.ImageSource.(Camera); ok {
		return c.NextPointCloud(ctx)
	}
	return nil, errors.New("source has no Projector/Camera Intrinsics associated with it to do a projection to a point cloud")
}

// ImageSourceWithProjector implements a CameraWithProjector with a gostream.ImageSource and Projector.
type imageSourceWithProjector struct {
	gostream.ImageSource
	rimage.Projector
}

// Close closes the underlying ImageSource.
func (iswp *imageSourceWithProjector) Close(ctx context.Context) error {
	return utils.TryClose(ctx, iswp.ImageSource)
}

// Projector returns the camera's Projector.
func (iswp *imageSourceWithProjector) GetProjector() rimage.Projector {
	return iswp.Projector
}

// NextPointCloud returns the next PointCloud from the camera, or will error if not supported.
func (iswp *imageSourceWithProjector) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	if c, ok := iswp.ImageSource.(Camera); ok {
		return c.NextPointCloud(ctx)
	}
	img, closer, err := iswp.Next(ctx)
	if err != nil {
		return nil, err
	}
	defer closer()
	return iswp.ImageWithDepthToPointCloud(rimage.ConvertToImageWithDepth(img))
}

// WrapWithReconfigurable wraps a camera with a reconfigurable and locking interface.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	c, ok := r.(Camera)
	if !ok {
		return nil, errors.Errorf("expected resource to be Camera but got %T", r)
	}
	if reconfigurable, ok := c.(*reconfigurableCamera); ok {
		return reconfigurable, nil
	}
	return &reconfigurableCamera{actual: c}, nil
}

var (
	_ = Camera(&reconfigurableCamera{})
	_ = resource.Reconfigurable(&reconfigurableCamera{})
)

type reconfigurableCamera struct {
	mu     sync.RWMutex
	actual Camera
}

func (c *reconfigurableCamera) ProxyFor() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual
}

func (c *reconfigurableCamera) Next(ctx context.Context) (image.Image, func(), error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.Next(ctx)
}

func (c *reconfigurableCamera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.NextPointCloud(ctx)
}

func (c *reconfigurableCamera) Close(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return utils.TryClose(ctx, c.actual)
}

// Reconfigure reconfigures the resource.
func (c *reconfigurableCamera) Reconfigure(ctx context.Context, newCamera resource.Reconfigurable) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	actual, ok := newCamera.(*reconfigurableCamera)
	if !ok {
		return errors.Errorf("expected new camera to be %T but got %T", c, newCamera)
	}
	if err := utils.TryClose(ctx, c.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	c.actual = actual.actual
	return nil
}
