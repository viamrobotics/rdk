package imagesource

import (
	"context"
	_ "embed" // for embedding camera parameters
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/core/artifact"
	"go.viam.com/core/rimage"
	"go.viam.com/core/rimage/transform"
)

//go:embed gripper_combo_parameters.json
var gripperjson []byte

func TestChangeCamera(t *testing.T) {
	logger := golog.NewTestLogger(t)

	iwd, err := rimage.ReadBothFromFile(artifact.MustPath("align/gripper1/align-test-1615761793.both.gz"), false)
	test.That(t, err, test.ShouldBeNil)

	// define the alignment and projection systems
	projectionSystem, err := transform.NewDepthColorIntrinsicsExtrinsicsFromBytes(gripperjson)
	test.That(t, err, test.ShouldBeNil)

	gripperConfig := &transform.AlignConfig{
		ColorInputSize:  image.Point{1024, 768},
		ColorWarpPoints: []image.Point{{604, 575}, {695, 115}},
		DepthInputSize:  image.Point{224, 171},
		DepthWarpPoints: []image.Point{{89, 109}, {206, 132}},
		OutputSize:      image.Point{448, 342},
		OutputOrigin:    image.Point{227, 160},
	}

	alignmentSystem, err := transform.NewDepthColorWarpTransforms(gripperConfig, logger)
	test.That(t, err, test.ShouldBeNil)

	// align the image
	iwd, err = alignmentSystem.AlignImageWithDepth(iwd)
	test.That(t, err, test.ShouldBeNil)

	// using alignment system
	test.That(t, iwd.CameraSystem(), test.ShouldHaveSameTypeAs, alignmentSystem)

	// change the camera system
	source := &StaticSource{iwd}
	csc := &CameraSystemChanger{source, projectionSystem}

	rawImage, _, err := csc.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	iwd2 := rimage.ConvertToImageWithDepth(rawImage)

	// using projection system
	test.That(t, iwd2.CameraSystem(), test.ShouldHaveSameTypeAs, projectionSystem)

}
