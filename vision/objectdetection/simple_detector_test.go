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
	// make the detector
	d := NewSimpleDetector(11, 15000)
	bbs, err := d.Inference(img)
	test.That(t, err, test.ShouldBeNil)
	// compare with expected bounding boxes
	test.That(t, bbs, test.ShouldHaveLength, 2)
	test.That(t, bbs[0], test.ShouldResemble, &Detection{image.Rect(0, 77, 110, 330), 1.0})
	test.That(t, bbs[1], test.ShouldResemble, &Detection{image.Rect(923, 229, 1129, 472), 1.0})
	// overlay the image and see if it is red where you expect
	ovImg := Overlay(img, bbs)
	test.That(t, ovImg.GetXY(0, 77), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(110, 330), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(923, 229), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(1129, 472), test.ShouldResemble, rimage.Red)
}
