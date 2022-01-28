package imagesource

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision/objectdetection"
)

func TestColorDetectionSource(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)
	source := &staticSource{img}

	cfg := &rimage.AttrConfig{Tolerance: 0.055556, SegmentSize: 15000, DetectColor: []uint8{79, 56, 21}, ExcludeColors: []string{"b"}}
	cameraSource, err := NewColorDetector(source, cfg)
	test.That(t, err, test.ShouldBeNil)
	detector := cameraSource.ImageSource.(*objectdetection.Source)
	defer detector.Close()

	resImg, _, err := detector.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	ovImg := rimage.ConvertImage(resImg)
	test.That(t, ovImg.GetXY(848, 424), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(999, 565), test.ShouldResemble, rimage.Red)
}

func BenchmarkColorDetectionSource(b *testing.B) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(b, err, test.ShouldBeNil)
	source := &staticSource{img}

	cfg := &rimage.AttrConfig{Tolerance: 0.055556, SegmentSize: 15000, DetectColor: []uint8{79, 56, 21}, ExcludeColors: []string{"b"}}
	cameraSource, err := NewColorDetector(source, cfg)
	test.That(b, err, test.ShouldBeNil)
	detector := cameraSource.ImageSource.(*objectdetection.Source)
	defer detector.Close()

	b.ResetTimer()
	// begin benchmarking
	for i := 0; i < b.N; i++ {
		_, _, _ = detector.Next(context.Background())
	}
}
