package fake

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/rimage/transform"
)

func TestFakeCameraHighResolution(t *testing.T) {
	camOri := &Camera{Name: "test_high", Model: fakeModelHigh}
	cam, err := camera.NewFromReader(context.Background(), camOri, fakeModelHigh, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	cameraTest(t, cam, 1280, 720, 812050, fakeIntrinsicsHigh, fakeDistortionHigh)
	err = cam.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestFakeCameraMedResolution(t *testing.T) {
	camOri := &Camera{Name: "test_med", Model: fakeModelMed, Resolution: "medium"}
	cam, err := camera.NewFromReader(context.Background(), camOri, fakeModelMed, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	cameraTest(t, cam, 640, 360, 203139, fakeIntrinsicsMed, nil)
	err = cam.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestFakeCameraLowResolution(t *testing.T) {
	camOri := &Camera{Name: "test_low", Model: fakeModelLow, Resolution: "low"}
	cam, err := camera.NewFromReader(context.Background(), camOri, fakeModelLow, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	cameraTest(t, cam, 320, 180, 50717, fakeIntrinsicsLow, nil)
	err = cam.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func cameraTest(
	t *testing.T,
	cam camera.Camera,
	width, height, points int,
	intrinsics *transform.PinholeCameraIntrinsics,
	distortion *transform.BrownConrady,
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
}
