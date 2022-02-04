package imagesource

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/rimage"
)

func TestColorDetectionSource(t *testing.T) {
	ctx, _ := context.WithCancel(context.Background())
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)
	source := &staticSource{img}
	cam, err := camera.New(source, nil, nil)
	test.That(t, err, test.ShouldBeNil)

	cfg := &camera.AttrConfig{Tolerance: 0.055556, SegmentSize: 15000, DetectColor: []uint8{79, 56, 21}, ExcludeColors: []string{"b"}}
	detector, err := newColorDetector(cam, cfg)
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, detector)

	resImg, _, err := detector.Next(ctx)
	test.That(t, err, test.ShouldBeNil)
	ovImg := rimage.ConvertImage(resImg)
	test.That(t, ovImg.GetXY(848, 424), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(999, 565), test.ShouldResemble, rimage.Red)
}

func BenchmarkColorDetectionSource(b *testing.B) {
	ctx, _ := context.WithCancel(context.Background())
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(b, err, test.ShouldBeNil)
	source := &staticSource{img}
	cam, err := camera.New(source, nil, nil)
	test.That(b, err, test.ShouldBeNil)

	cfg := &camera.AttrConfig{Tolerance: 0.055556, SegmentSize: 15000, DetectColor: []uint8{79, 56, 21}, ExcludeColors: []string{"b"}}
	detector, err := newColorDetector(cam, cfg)
	test.That(b, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, detector)

	b.ResetTimer()
	// begin benchmarking
	for i := 0; i < b.N; i++ {
		_, _, _ = detector.Next(ctx)
	}
}
