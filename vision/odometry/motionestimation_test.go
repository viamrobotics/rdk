package odometry

import (
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/rimage"
)

func TestNewMotion3DFromRotationTranslation(t *testing.T) {
	// rotation = Id
	rot := mat.NewDense(3, 3, nil)
	rot.Set(0, 0, 1)
	rot.Set(1, 1, 1)
	rot.Set(2, 2, 1)

	// Translation = 1m in z direction
	tr := mat.NewDense(3, 1, []float64{0, 0, 1})

	motion := NewMotion3DFromRotationTranslation(rot, tr)
	test.That(t, motion, test.ShouldNotBeNil)
	test.That(t, motion.Rotation, test.ShouldResemble, rot)
	test.That(t, motion.Translation, test.ShouldResemble, tr)
}

func TestEstimateMotionFrom2Frames(t *testing.T) {
	logger := golog.NewTestLogger(t)
	// load cfg
	cfg := LoadMotionEstimationConfig("vo_config.json")
	// load images
	im1, err := rimage.NewImageFromFile(artifact.MustPath("vision/odometry/000001.png"))
	test.That(t, err, test.ShouldBeNil)
	im2, err := rimage.NewImageFromFile(artifact.MustPath("vision/odometry/000002.png"))
	test.That(t, err, test.ShouldBeNil)
	// Estimate motion
	motion, _, err := EstimateMotionFrom2Frames(im1, im2, cfg, logger, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, motion.Translation.At(2, 0), test.ShouldBeLessThan, -1.0)
	test.That(t, motion.Translation.At(1, 0), test.ShouldBeLessThan, 0.2)
}
