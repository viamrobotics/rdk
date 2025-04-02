package objectdetection

import (
	"image"
	"testing"

	"go.viam.com/test"
)

// TestLoadModel validates that the model loads successfully.
func TestNewDetector(t *testing.T) {
	det := NewDetection(image.Rect(0, 0, 100, 100), image.Rect(0, 0, 30, 30), 0.5, "A")
	test.That(t, det, test.ShouldNotBeNil)
	test.That(t, det.Label(), test.ShouldEqual, "A")
	test.That(t, det.Score(), test.ShouldEqual, 0.5)
	test.That(t, *det.BoundingBox(), test.ShouldResemble, image.Rect(0, 0, 30, 30))
	test.That(t, det.NormalizedBoundingBox(), test.ShouldResemble, []float64{0.0, 0.0, 0.3, 0.3})

	det2 := NewDetectionWithoutImgBounds(image.Rect(0, 0, 30, 30), 0.6, "B")
	test.That(t, det2, test.ShouldNotBeNil)
	test.That(t, det2.NormalizedBoundingBox(), test.ShouldBeNil)
	test.That(t, det2.Label(), test.ShouldEqual, "B")
	test.That(t, det2.Score(), test.ShouldEqual, 0.6)
	test.That(t, *det2.BoundingBox(), test.ShouldResemble, image.Rect(0, 0, 30, 30))
}
