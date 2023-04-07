package fake

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/rimage/transform"
)

func TestFakeCameraHighResolution(t *testing.T) {
	model, width, height := fakeModel(1280, 720)
	camOri := &Camera{Name: "test_high", Model: model, Width: width, Height: height}
	cam, err := camera.NewFromReader(context.Background(), camOri, model, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	cameraTest(t, cam, 1280, 720, 921600, model.PinholeCameraIntrinsics, model.Distortion)
	// (0,0) entry defaults to (1280, 720)
	model, width, height = fakeModel(0, 0)
	camOri = &Camera{Name: "test_high_zero", Model: model, Width: width, Height: height}
	cam, err = camera.NewFromReader(context.Background(), camOri, model, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	cameraTest(t, cam, 1280, 720, 921600, model.PinholeCameraIntrinsics, model.Distortion)
}

func TestFakeCameraMedResolution(t *testing.T) {
	model, width, height := fakeModel(640, 360)
	camOri := &Camera{Name: "test_high", Model: model, Width: width, Height: height}
	cam, err := camera.NewFromReader(context.Background(), camOri, model, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	cameraTest(t, cam, 640, 360, 230400, model.PinholeCameraIntrinsics, model.Distortion)
	err = cam.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestFakeCameraUnspecified(t *testing.T) {
	// one unspecified side should keep 16:9 aspect ratio
	// (320, 0) -> (320, 180)
	model, width, height := fakeModel(320, 0)
	camOri := &Camera{Name: "test_320", Model: model, Width: width, Height: height}
	cam, err := camera.NewFromReader(context.Background(), camOri, model, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	cameraTest(t, cam, 320, 180, 57600, model.PinholeCameraIntrinsics, model.Distortion)
	// (0, 180) -> (320, 180)
	model, width, height = fakeModel(0, 180)
	camOri = &Camera{Name: "test_180", Model: model, Width: width, Height: height}
	cam, err = camera.NewFromReader(context.Background(), camOri, model, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	cameraTest(t, cam, 320, 180, 57600, model.PinholeCameraIntrinsics, model.Distortion)
}

func TestFakeCameraParams(t *testing.T) {
	// test odd width and height
	cfg := &Attrs{
		Width:  321,
		Height: 0,
	}
	err := cfg.Validate()
	test.That(t, err, test.ShouldNotBeNil)
	cfg = &Attrs{
		Width:  0,
		Height: 321,
	}
	err = cfg.Validate()
	test.That(t, err, test.ShouldNotBeNil)
}

func cameraTest(
	t *testing.T,
	cam camera.Camera,
	width, height, points int,
	intrinsics *transform.PinholeCameraIntrinsics,
	distortion transform.Distorter,
) {
	t.Helper()
	stream, err := cam.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)
	img, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img.Bounds().Dx(), test.ShouldEqual, width)
	test.That(t, img.Bounds().Dy(), test.ShouldEqual, height)
	pc, err := cam.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, points)
	prop, err := cam.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, prop.IntrinsicParams, test.ShouldResemble, intrinsics)
	if distortion == nil {
		test.That(t, prop.DistortionParams, test.ShouldBeNil)
	} else {
		test.That(t, prop.DistortionParams, test.ShouldResemble, distortion)
	}
	err = cam.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}
