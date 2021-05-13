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
	iwd, err := rimage.BothReadFromFile(artifact.MustPath("align/gripper1/align-test-1615761790.both.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, iwd.IsAligned(), test.ShouldEqual, false)

	// from robots/config/gripper-cam.json
	config := &AlignConfig{
		ColorInputSize:  image.Point{1024, 768},
		ColorWarpPoints: []image.Point{{604, 575}, {695, 115}},
		DepthInputSize:  image.Point{224, 171},
		DepthWarpPoints: []image.Point{{89, 109}, {206, 132}},
		OutputSize:      image.Point{448, 342},
		Smooth:          true,
	}
	dct, err := NewDepthColorWarpTransforms(config, logger)
	test.That(t, err, test.ShouldBeNil)

	pc, err := dct.ImageWithDepthToPointCloud(iwd)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc, test.ShouldNotBeNil)
	// the underlying iwd was not changed
	test.That(t, iwd.IsAligned(), test.ShouldEqual, false)
	test.That(t, iwd.GetCameraSystem(), test.ShouldBeNil)

	// image with depth with depth missing should return error
	img, err := rimage.NewImageFromFile(artifact.MustPath("align/gripper1/align-test-1615761790.both.gz"))
	test.That(t, err, test.ShouldBeNil)

	iwdBad := rimage.MakeImageWithDepth(img, nil, false, nil)
	pcBad, err := dct.ImageWithDepthToPointCloud(iwdBad)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, pcBad, test.ShouldBeNil)
	test.That(t, iwdBad.IsAligned(), test.ShouldEqual, false)

}
