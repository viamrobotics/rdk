package objectdetection

import (
	"context"
	"image"
	"testing"

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

func TestSimpleDetection(t *testing.T) {
	// make the original source
	src := &simpleSource{artifact.MustPath("vision/objectdetection/detection_test.jpg")}
	// make the preprocessing function
	rb, err := RemoveColorChannel("b")
	test.That(t, err, test.ShouldBeNil)
	p := func(img image.Image) image.Image {
		return rb(CopyImage(img))
	}
	// make the detector
	d := NewSimpleDetector(10.)
	// make the filter
	f := NewAreaFilter(15000)

	// Make the detection source
	pipeline, err := NewSource(src, p, d, f)
	test.That(t, err, test.ShouldBeNil)

	// compare with expected bounding boxes
	res, err := pipeline.NextResult(context.Background())
	test.That(t, err, test.ShouldBeNil)
	bbs := res.Detections
	test.That(t, bbs, test.ShouldHaveLength, 2)
	test.That(t, bbs[0].Score(), test.ShouldEqual, 1.0)
	test.That(t, bbs[0].BoundingBox(), test.ShouldResemble, &image.Rectangle{image.Point{0, 77}, image.Point{110, 330}})
	test.That(t, bbs[1].Score(), test.ShouldEqual, 1.0)
	test.That(t, bbs[1].BoundingBox(), test.ShouldResemble, &image.Rectangle{image.Point{963, 349}, image.Point{1129, 472}})

	// overlay the image and see if it is red where you expect
	img, _, err := pipeline.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	ovImg := rimage.ConvertImage(img)
	test.That(t, ovImg.GetXY(0, 77), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(110, 330), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(963, 349), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(1129, 472), test.ShouldResemble, rimage.Red)
}
