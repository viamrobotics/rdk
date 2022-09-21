package fake

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
)

func TestFakeCamera(t *testing.T) {
	camOri := &Camera{Name: "test", Model: fakeModel}
	cam, err := camera.NewFromReader(context.Background(), camOri, fakeModel, camera.ColorStream)
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
	test.That(t, prop.IntrinsicParams, test.ShouldResemble, fakeIntrinsics)
	test.That(t, prop.DistortionParams, test.ShouldResemble, fakeDistortion)
	err = cam.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}
