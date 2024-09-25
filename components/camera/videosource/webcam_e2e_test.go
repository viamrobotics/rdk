//go:build linux && vcamera

package videosource_test

import (
	"context"
	"testing"

	pb "go.viam.com/api/component/camera/v1"
	"go.viam.com/rdk/logging"
	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/videosource"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/vcamera"
)

func findWebcam(t *testing.T, webcams []*pb.Webcam, name string) *pb.Webcam {
	t.Helper()
	for _, w := range webcams {
		if w.Name == name {
			return w
		}
	}
	t.Fatalf("could not find webcam %s", name)
	return nil
}

func TestWebcamDiscovery(t *testing.T) {
	logger := logging.NewTestLogger(t)

	reg, ok := resource.LookupRegistration(camera.API, videosource.ModelWebcam)
	test.That(t, ok, test.ShouldBeTrue)

	ctx := context.Background()
	discoveries, err := reg.Discover(ctx, logger)
	test.That(t, err, test.ShouldBeNil)

	webcams, ok := discoveries.(*pb.Webcams)
	test.That(t, ok, test.ShouldBeTrue)
	webcamsLen := len(webcams.Webcams)

	// Video capture and overlay minor numbers range == [0, 63]
	// Start from the end of the range to avoid conflicts with other devices
	// Source: https://www.kernel.org/doc/html/v4.9/media/uapi/v4l/diff-v4l.html
	config, err := vcamera.Builder(logger).
		NewCamera(62, "Lo Res Webcam", vcamera.Resolution{Width: 640, Height: 480}).
		NewCamera(63, "Hi Res Webcam", vcamera.Resolution{Width: 1280, Height: 720}).
		Stream()

	test.That(t, err, test.ShouldBeNil)
	defer config.Shutdown()

	discoveries, err = reg.Discover(ctx, logger)
	test.That(t, err, test.ShouldBeNil)

	webcams, ok = discoveries.(*pb.Webcams)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, len(webcams.Webcams), test.ShouldEqual, webcamsLen+2)

	webcam := findWebcam(t, webcams.Webcams, "Hi Res Webcam")
	test.That(t, webcam.Properties[0].WidthPx, test.ShouldEqual, 1280)
	test.That(t, webcam.Properties[0].HeightPx, test.ShouldEqual, 720)

	webcam = findWebcam(t, webcams.Webcams, "Lo Res Webcam")
	test.That(t, webcam.Properties[0].WidthPx, test.ShouldEqual, 640)
	test.That(t, webcam.Properties[0].HeightPx, test.ShouldEqual, 480)
}

func newWebcamConfig(name, path string) resource.Config {
	conf := resource.NewEmptyConfig(
		resource.NewName(camera.API, name),
		resource.DefaultModelFamily.WithModel("webcam"),
	)
	conf.ConvertedAttributes = &videosource.WebcamConfig{Path: path}
	return conf
}

func TestWebcamGetImage(t *testing.T) {
	logger := logging.NewTestLogger(t)
	config, err := vcamera.Builder(logger).
		NewCamera(62, "Lo Res Webcam", vcamera.Resolution{Width: 640, Height: 480}).
		NewCamera(63, "Hi Res Webcam", vcamera.Resolution{Width: 1280, Height: 720}).
		Stream()

	test.That(t, err, test.ShouldBeNil)
	defer config.Shutdown()

	cancelCtx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	conf := newWebcamConfig("cam1", "video62")
	cam1, err := videosource.NewWebcam(cancelCtx, nil, conf, logger)
	test.That(t, err, test.ShouldBeNil)
	defer cam1.Close(cancelCtx)

	stream, err := cam1.Stream(cancelCtx)
	test.That(t, err, test.ShouldBeNil)

	img, rel, err := stream.Next(cancelCtx)
	test.That(t, err, test.ShouldBeNil)
	defer rel()

	test.That(t, img.Bounds().Dx(), test.ShouldEqual, 640)
	test.That(t, img.Bounds().Dy(), test.ShouldEqual, 480)

	conf = newWebcamConfig("cam2", "video63")
	cam2, err := videosource.NewWebcam(cancelCtx, nil, conf, logger)
	test.That(t, err, test.ShouldBeNil)
	defer cam2.Close(cancelCtx)

	stream, err = cam2.Stream(cancelCtx)
	test.That(t, err, test.ShouldBeNil)

	img, rel, err = stream.Next(cancelCtx)
	test.That(t, err, test.ShouldBeNil)
	defer rel()

	test.That(t, img.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, img.Bounds().Dy(), test.ShouldEqual, 720)
}
