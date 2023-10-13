//go:build !no_media

package videosource_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"

	"go.viam.com/rdk/components/camera/videosource"
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
	logger := golog.NewTestLogger(t)
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
