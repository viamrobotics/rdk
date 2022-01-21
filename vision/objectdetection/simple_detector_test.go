package objectdetection

import (
	"image"
	"testing"

	"go.viam.com/rdk/rimage"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestSimpleDetection(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)

	// make a preprocessing function
	rb := RemoveBlue()
	// make the detector
	d := NewSimpleDetector(10)
	// make the filter
	f := NewAreaFilter(15000)

	// apply the pipeline
	rimg := rb(img)
	bbs, err := d(rimg)
	test.That(t, err, test.ShouldBeNil)
	bbs = f(bbs)

	// compare with expected bounding boxes
	test.That(t, bbs, test.ShouldHaveLength, 2)
	test.That(t, bbs[0].Score(), test.ShouldEqual, 1.0)
	test.That(t, bbs[0].BoundingBox(), test.ShouldResemble, &image.Rectangle{image.Point{0, 77}, image.Point{110, 330}})
	test.That(t, bbs[1].Score(), test.ShouldEqual, 1.0)
	test.That(t, bbs[1].BoundingBox(), test.ShouldResemble, &image.Rectangle{image.Point{963, 349}, image.Point{1129, 472}})
	// overlay the image and see if it is red where you expect
	ovImg := Overlay(img, bbs)
	test.That(t, ovImg.GetXY(0, 77), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(110, 330), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(963, 349), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(1129, 472), test.ShouldResemble, rimage.Red)
}
