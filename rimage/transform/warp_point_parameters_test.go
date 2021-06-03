package transform

import (
	"image"
	"testing"

	"go.viam.com/core/artifact"
	"go.viam.com/core/rimage"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestImageWithDepthToPointCloud(t *testing.T) {
	logger := golog.NewTestLogger(t)
	iwd, err := rimage.ReadBothFromFile(artifact.MustPath("align/gripper1/align-test-1615761790.both.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, iwd.IsAligned(), test.ShouldEqual, false)

	// from robots/config/gripper-cam.json
	config := &AlignConfig{
		ColorInputSize:  image.Point{1024, 768},
		ColorWarpPoints: []image.Point{{604, 575}, {695, 115}},
		DepthInputSize:  image.Point{224, 171},
		DepthWarpPoints: []image.Point{{89, 109}, {206, 132}},
		OutputSize:      image.Point{448, 342},
	}
	dct, err := NewDepthColorWarpTransforms(config, logger)
	test.That(t, err, test.ShouldBeNil)

	pc, err := dct.ImageWithDepthToPointCloud(iwd)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc, test.ShouldNotBeNil)
	// the underlying iwd was not changed
	test.That(t, iwd.IsAligned(), test.ShouldEqual, false)
	test.That(t, iwd.CameraSystem(), test.ShouldBeNil)

	// image with depth with depth missing should return error
	img, err := rimage.NewImageFromFile(artifact.MustPath("align/gripper1/align-test-1615761790.both.gz"))
	test.That(t, err, test.ShouldBeNil)

	iwdBad := rimage.MakeImageWithDepth(img, nil, false, nil)
	pcBad, err := dct.ImageWithDepthToPointCloud(iwdBad)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, pcBad, test.ShouldBeNil)
	test.That(t, iwdBad.IsAligned(), test.ShouldEqual, false)

}
func TestWarpPointsTo3D(t *testing.T) {
	logger := golog.NewTestLogger(t)
	iwd, err := rimage.ReadBothFromFile(artifact.MustPath("align/gripper1/align-test-1615761790.both.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, iwd.IsAligned(), test.ShouldEqual, false)

	// from robots/config/gripper-cam.json
	config := &AlignConfig{
		ColorInputSize:  image.Point{1024, 768},
		ColorWarpPoints: []image.Point{{604, 575}, {695, 115}},
		DepthInputSize:  image.Point{224, 171},
		DepthWarpPoints: []image.Point{{89, 109}, {206, 132}},
		OutputSize:      image.Point{448, 342},
	}
	testPoint := image.Point{0, 0}
	assertTo3DPanic(t, iwd, testPoint)

	dct, err := NewDepthColorWarpTransforms(config, logger)
	test.That(t, err, test.ShouldBeNil)
	// align the image now
	iwd, err = dct.AlignImageWithDepth(iwd)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, iwd.IsAligned(), test.ShouldEqual, true)
	// test out To3D
	vec := iwd.To3D(testPoint)
	test.That(t, vec.Z, test.ShouldEqual, float64(iwd.Depth.Get(testPoint)))
	// out of bounds - panic
	testPoint = image.Point{iwd.Width(), iwd.Height()}
	assertTo3DPanic(t, iwd, testPoint)
}

func assertTo3DPanic(t *testing.T, iwd *rimage.ImageWithDepth, point image.Point) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("The code did not panic")
		}
	}()
	iwd.To3D(point)
}
