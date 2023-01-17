package fake

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
)

func TestFakeCameraHighResolution(t *testing.T) {
	camOri := &Camera{Name: "test_high", Model: fakeModelHigh}
	cam, err := camera.NewFromReader(context.Background(), camOri, fakeModelHigh, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	stream, err := cam.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)
	img, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, img.Bounds().Dy(), test.ShouldEqual, 720)
	pc, err := cam.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 812050)
	prop, err := cam.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, prop.IntrinsicParams, test.ShouldResemble, fakeIntrinsicsHigh)
	test.That(t, prop.DistortionParams, test.ShouldResemble, fakeDistortionHigh)
	err = cam.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestFakeCameraMedResolution(t *testing.T) {
	camOri := &Camera{Name: "test_med", Model: fakeModelMed, Resolution: "medium"}
	cam, err := camera.NewFromReader(context.Background(), camOri, fakeModelMed, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	stream, err := cam.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)
	img, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img.Bounds().Dx(), test.ShouldEqual, 640)
	test.That(t, img.Bounds().Dy(), test.ShouldEqual, 360)
	pc, err := cam.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 206957)
	prop, err := cam.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, prop.IntrinsicParams, test.ShouldResemble, fakeIntrinsicsMed)
	test.That(t, prop.DistortionParams, test.ShouldBeNil)
	err = cam.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestFakeCameraLowResolution(t *testing.T) {
	camOri := &Camera{Name: "test_low", Model: fakeModelLow, Resolution: "low"}
	cam, err := camera.NewFromReader(context.Background(), camOri, fakeModelLow, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	stream, err := cam.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)
	img, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img.Bounds().Dx(), test.ShouldEqual, 320)
	test.That(t, img.Bounds().Dy(), test.ShouldEqual, 180)
	pc, err := cam.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 52918)
	prop, err := cam.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, prop.IntrinsicParams, test.ShouldResemble, fakeIntrinsicsLow)
	test.That(t, prop.DistortionParams, test.ShouldBeNil)
	err = cam.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}
