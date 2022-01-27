package imagesource

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision/objectdetection"
)

func TestSimpleDetectionSource(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)
	source := &staticSource{img}

	cfg := &rimage.AttrConfig{Threshold: 10.0, SegmentSize: 15000}
	cameraSource, err := NewSimpleObjectDetector(source, cfg)
	test.That(t, err, test.ShouldBeNil)
	detector := cameraSource.ImageSource.(*objectdetection.Source)
	defer detector.Close()

	resImg, _, err := detector.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	ovImg := rimage.ConvertImage(resImg)
	test.That(t, ovImg.GetXY(0, 77), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(110, 330), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(963, 349), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(1129, 472), test.ShouldResemble, rimage.Red)
}

func BenchmarkSimpleDetectionSource(b *testing.B) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(b, err, test.ShouldBeNil)
	source := &staticSource{img}

	cfg := &rimage.AttrConfig{Threshold: 10.0, SegmentSize: 15000}
	cameraSource, err := NewSimpleObjectDetector(source, cfg)
	test.That(b, err, test.ShouldBeNil)
	detector := cameraSource.ImageSource.(*objectdetection.Source)
	defer detector.Close()

	b.ResetTimer()
	// begin benchmarking
	for i := 0; i < b.N; i++ {
		_, _, _ = detector.Next(context.Background())
	}
}
