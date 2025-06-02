package fake

import (
	"context"
	"image"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/testutils/inject"
)

func TestFakeVision(t *testing.T) {
	ctx := context.Background()
	r := &inject.Robot{}
	name := vision.Named("test_fake")
	srv, err := registerFake(name, r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, srv.Name(), test.ShouldResemble, name)
	img := image.NewRGBA(image.Rect(0, 0, 100, 200))

	// Test properties
	props, err := srv.GetProperties(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.DetectionSupported, test.ShouldEqual, true)
	test.That(t, props.ClassificationSupported, test.ShouldEqual, true)
	test.That(t, props.ObjectPCDsSupported, test.ShouldEqual, false)

	// Test Detections
	det, err := srv.Detections(ctx, img, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, det, test.ShouldHaveLength, 1)
	test.That(t, det[0].Score(), test.ShouldAlmostEqual, fakeDetScore)
	test.That(t, det[0].Label(), test.ShouldEqual, fakeDetLabel)
	bounds := image.Rect(25, 50, 75, 150)
	test.That(t, det[0].BoundingBox(), test.ShouldResemble, &bounds)

	// Test Classifications
	cls, err := srv.Classifications(ctx, img, 1, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cls, test.ShouldHaveLength, 1)
	test.That(t, cls[0].Score(), test.ShouldAlmostEqual, fakeClassScore)
	test.That(t, cls[0].Label(), test.ShouldEqual, fakeClassLabel)
}
