package objectdetection

import (
	"context"
	"image"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
)

type simpleSource struct {
	filePath string
}

func (s *simpleSource) Next(ctx context.Context) (image.Image, func(), error) {
	img, err := rimage.NewImageFromFile(s.filePath)
	return img, func() {}, err
}

func TestColorDetection(t *testing.T) {
	// make the original source
	src := &simpleSource{artifact.MustPath("vision/objectdetection/detection_test.jpg")}
	// make the preprocessing function
	p, err := RemoveColorChannel("b")
	test.That(t, err, test.ShouldBeNil)
	// make the detector
	theColor := rimage.NewColor(79, 56, 21) // a yellow color
	hue, _, _ := theColor.HsvNormal()
	_, err = NewColorDetector(8., hue)
	test.That(t, err, test.ShouldBeError, errors.New("tolerance must be between 0.0 and 1.0. Got 8.00000"))
	d, err := NewColorDetector(0.0444444444, hue)
	test.That(t, err, test.ShouldBeNil)
	// make the filter
	f := NewAreaFilter(15000)

	// Make the detection source
	pipeline, err := NewSource(src, p, d, f)
	test.That(t, err, test.ShouldBeNil)
	defer pipeline.Close()

	// compare with expected bounding boxes
	res, err := pipeline.NextResult(context.Background())
	test.That(t, err, test.ShouldBeNil)
	bbs := res.Detections
	test.That(t, bbs, test.ShouldHaveLength, 1)
	test.That(t, bbs[0].Score(), test.ShouldEqual, 1.0)
	test.That(t, bbs[0].BoundingBox(), test.ShouldResemble, &image.Rectangle{image.Point{848, 424}, image.Point{999, 565}})

	// overlay the image and see if it is red where you expect
	img, _, err := pipeline.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	ovImg := rimage.ConvertImage(img)
	test.That(t, ovImg.GetXY(848, 424), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(999, 565), test.ShouldResemble, rimage.Red)
}
