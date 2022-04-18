package objectdetection

import (
	"image"
	"testing"

	"go.viam.com/test"

	objdet "go.viam.com/rdk/vision/objectdetection"
)

func TestDetectorRegistry(t *testing.T) {
	fn := func(image.Image) ([]objdet.Detection, error) {
		return []objdet.Detection{objdet.NewDetection(image.Rectangle{}, 0.0, "")}, nil
	}
	fnName := "x"
	// no detector
	test.That(t, func() { RegisterDetector(fnName, nil) }, test.ShouldPanic)
	// success
	RegisterDetector(fnName, fn)
	// detector names
	names := DetectorNames()
	test.That(t, names, test.ShouldNotBeNil)
	test.That(t, names, test.ShouldContain, fnName)
	// look up
	det, err := DetectorLookup(fnName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, det, test.ShouldEqual, fn)
	det, err = DetectorLookup("z")
	test.That(t, err.Error(), test.ShouldContainSubstring, "no Detector with name")
	test.That(t, det, test.ShouldBeNil)
	// duplicate
	test.That(t, func() { RegisterDetector(fnName, fn) }, test.ShouldPanic)
}
