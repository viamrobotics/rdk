package objectdetection_test

import (
	"context"
	"image"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera/imagesource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision/objectdetection"
)

func TestDetectionSource(t *testing.T) {
	// make the original source
	sourceImg, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)
	src := &imagesource.StaticSource{sourceImg}
	// make the preprocessing function
	p, err := objectdetection.RemoveColorChannel("b")
	test.That(t, err, test.ShouldBeNil)
	// make the detector
	detCfg := &objectdetection.ColorDetectorConfig{
		SegmentSize:       15000,
		Tolerance:         0.0444444,
		DetectColorString: "#4f3815",
	}
	d, err := objectdetection.NewColorDetector(detCfg)
	test.That(t, err, test.ShouldBeNil)
	// Make the detection source
	det, err := objectdetection.Build(p, d, nil)
	test.That(t, err, test.ShouldBeNil)
	pipeline, err := objectdetection.NewSource(src, det)
	test.That(t, err, test.ShouldBeNil)
	defer pipeline.Close()

	// compare with expected bounding boxes
	res, err := pipeline.NextResult(context.Background())
	test.That(t, err, test.ShouldBeNil)
	bbs := res.Detections
	test.That(t, bbs, test.ShouldHaveLength, 1)
	test.That(t, bbs[0].Score(), test.ShouldEqual, 1.0)
	test.That(t, bbs[0].Label(), test.ShouldEqual, "orange")
	test.That(t, bbs[0].BoundingBox(), test.ShouldResemble, &image.Rectangle{image.Point{848, 424}, image.Point{999, 565}})

	// overlay the image and see if it is red where you expect
	img, _, err := pipeline.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	ovImg := rimage.ConvertImage(img)
	test.That(t, ovImg.GetXY(848, 424), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(998, 564), test.ShouldResemble, rimage.Red)
}

func TestEmptyDetection(t *testing.T) {
	d := objectdetection.NewDetection(image.Rectangle{}, 0., "")
	test.That(t, d.Score(), test.ShouldEqual, 0.0)
	test.That(t, d.Label(), test.ShouldEqual, "")
	test.That(t, d.BoundingBox(), test.ShouldResemble, &image.Rectangle{})
}
