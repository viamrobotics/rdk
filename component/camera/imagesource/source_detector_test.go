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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)
	source := &StaticSource{img}
	cam, err := camera.New(source, nil, nil)
	test.That(t, err, test.ShouldBeNil)

	cfg := &colorDetectorAttrs{Tolerance: 0.055556, SegmentSize: 15000, DetectColorString: "#4F3815", ExcludeColors: []string{"b"}}
	detector, err := newColorDetector(cam, cfg)
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, detector)

	resImg, _, err := detector.Next(ctx)
	test.That(t, err, test.ShouldBeNil)
	ovImg := rimage.ConvertImage(resImg)
	test.That(t, ovImg.GetXY(848, 424), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(999, 565), test.ShouldResemble, rimage.Red)
}

func TestDetectColor(t *testing.T) {
	// empty string
	attrs := &colorDetectorAttrs{DetectColorString: ""}
	result, err := attrs.DetectColor()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldHaveLength, 0)
	// not a pound sign
	attrs = &colorDetectorAttrs{DetectColorString: "$121CFF"}
	_, err = attrs.DetectColor()
	test.That(t, err.Error(), test.ShouldContainSubstring, "detect_color is ill-formed")
	// string too long
	attrs = &colorDetectorAttrs{DetectColorString: "#121CFF03"}
	_, err = attrs.DetectColor()
	test.That(t, err.Error(), test.ShouldContainSubstring, "detect_color is ill-formed")
	// string too short
	attrs = &colorDetectorAttrs{DetectColorString: "#121C"}
	_, err = attrs.DetectColor()
	test.That(t, err.Error(), test.ShouldContainSubstring, "detect_color is ill-formed")
	// not a hex string
	attrs = &colorDetectorAttrs{DetectColorString: "#1244GG"}
	_, err = attrs.DetectColor()
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid byte")
	// success
	attrs = &colorDetectorAttrs{DetectColorString: "#1244FF"}
	result, err = attrs.DetectColor()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldHaveLength, 3)
	test.That(t, result[0], test.ShouldEqual, 18)
	test.That(t, result[1], test.ShouldEqual, 68)
	test.That(t, result[2], test.ShouldEqual, 255)
}

func BenchmarkColorDetectionSource(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(b, err, test.ShouldBeNil)
	source := &StaticSource{img}
	cam, err := camera.New(source, nil, nil)
	test.That(b, err, test.ShouldBeNil)

	cfg := &colorDetectorAttrs{Tolerance: 0.055556, SegmentSize: 15000, DetectColorString: "#4F3815", ExcludeColors: []string{"b"}}
	detector, err := newColorDetector(cam, cfg)
	test.That(b, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, detector)

	b.ResetTimer()
	// begin benchmarking
	for i := 0; i < b.N; i++ {
		_, _, _ = detector.Next(ctx)
	}
}
