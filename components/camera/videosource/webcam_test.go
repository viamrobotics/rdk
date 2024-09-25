package videosource_test

import (
	"context"
	"image"
	"testing"

	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/videosource"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
)

// fakeDriver is a driver has a label and media properties.
type fakeDriver struct {
	label string
	props []prop.Media
}

func (d *fakeDriver) Open() error              { return nil }
func (d *fakeDriver) Properties() []prop.Media { return d.props }
func (d *fakeDriver) ID() string               { return d.label }
func (d *fakeDriver) Info() driver.Info        { return driver.Info{Label: d.label} }
func (d *fakeDriver) Status() driver.State     { return "some state" }
func (d *fakeDriver) Close() error             { return nil }

func newFakeDriver(label string, props []prop.Media) driver.Driver {
	return &fakeDriver{label: label, props: props}
}

func testGetDrivers() []driver.Driver {
	props := prop.Media{
		Video: prop.Video{Width: 320, Height: 240, FrameFormat: "some format", FrameRate: 30.0},
	}
	withProps := newFakeDriver("some label", []prop.Media{props})
	withoutProps := newFakeDriver("another label", []prop.Media{})
	return []driver.Driver{withProps, withoutProps}
}

func TestDiscoveryWebcam(t *testing.T) {
	logger := logging.NewTestLogger(t)
	resp, err := videosource.Discover(context.Background(), testGetDrivers, logger)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Webcams, test.ShouldHaveLength, 1)
	test.That(t, resp.Webcams[0].Label, test.ShouldResemble, "some label")
	test.That(t, resp.Webcams[0].Status, test.ShouldResemble, "some state")

	respProps := resp.Webcams[0].Properties
	test.That(t, respProps, test.ShouldHaveLength, 1)
	test.That(t, respProps[0].WidthPx, test.ShouldResemble, int32(320))
	test.That(t, respProps[0].HeightPx, test.ShouldResemble, int32(240))
	test.That(t, respProps[0].FrameFormat, test.ShouldResemble, "some format")
	test.That(t, respProps[0].FrameRate, test.ShouldResemble, float32(30))
}

// Testing FrameRate camera property

type mockVideoSource struct {
	frameRate float32
}

func (m *mockVideoSource) Properties(context.Context) (camera.Properties, error) {
	return camera.Properties{FrameRate: m.frameRate}, nil
}
func (m *mockVideoSource) Close(ctx context.Context) error {
	return nil
}
func (m *mockVideoSource) Images(context.Context) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	return nil, resource.ResponseMetadata{}, nil
}
func (m *mockVideoSource) NextPointCloud(context.Context) (pointcloud.PointCloud, error) {
	return nil, nil
}
func (m *mockVideoSource) Stream(context.Context, ...gostream.ErrorHandler) (gostream.MediaStream[image.Image], error) {
	return nil, nil
}

type fakeMonitoredWebcam struct {
	resource.Named
	exposedProjector camera.VideoSource
}

func TestFrameRate(t *testing.T) {
	mockCam := &mockVideoSource{frameRate: 0.0}

	fakeCam := &fakeMonitoredWebcam{
		exposedProjector: mockCam,
	}
	// Test without setting frame rate
	props, err := fakeCam.exposedProjector.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.FrameRate, test.ShouldEqual, 0.0)

	// Test with setting frame rate
	fakeFrameRate := float32(30.0)
	mockCam.frameRate = fakeFrameRate
	props, err = fakeCam.exposedProjector.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.FrameRate, test.ShouldEqual, fakeFrameRate)

	defer mockCam.Close(context.Background())
}
