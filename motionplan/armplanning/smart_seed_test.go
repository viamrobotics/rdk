package armplanning

import (
	"testing"
	"time"
	
	"github.com/golang/geo/r3"
	
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
	"go.viam.com/test"
)

func TestSmartSeedCache1(t *testing.T) {
	logger := logging.NewTestLogger(t)

	armName := "ur5e"
	armKinematics, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), armName)
	test.That(t, err, test.ShouldBeNil)

	fs := referenceframe.NewEmptyFrameSystem("pirouette")
	err = fs.AddFrame(armKinematics, fs.World())
	test.That(t, err, test.ShouldBeNil)

	c, err := smartSeed(fs)
	test.That(t, err, test.ShouldBeNil)

	start := referenceframe.FrameSystemInputs{"ur5e" : {{1.0471667423817}, {0.011108350341463286}, {-1.0899013011625651}, {-3.0938870331059594}, {-1.767558957758243e-05}, {-3.681258507284093}}}

	startTime := time.Now()
	seed, err := c.findSeed(
		referenceframe.FrameSystemPoses{"ur5e": referenceframe.NewPoseInFrame("world",
			spatialmath.NewPose(
				r3.Vector{X:-337.976430, Y:-464.051182, Z:554.695381},
				&spatialmath.OrientationVectorDegrees{OX:0.499987, OY:-0.866033, OZ:-0.000000, Theta:0.000000},
			))},
		start, logger)
	test.That(t, err, test.ShouldBeNil)
	logger.Infof("time to run findSeed: %v", time.Since(startTime))
	
	cost := referenceframe.InputsL2Distance(start["ur5e"], seed["ur5e"])
	test.That(t, cost, test.ShouldBeLessThan, 1.25)

}

func BenchmarkSmartSeedCacheSearch(t *testing.B) {
	logger := logging.NewTestLogger(t)
	
	armName := "ur5e"
	armKinematics, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), armName)
	test.That(t, err, test.ShouldBeNil)

	fs := referenceframe.NewEmptyFrameSystem("pirouette")
	err = fs.AddFrame(armKinematics, fs.World())
	test.That(t, err, test.ShouldBeNil)

	c, err := smartSeed(fs)
	test.That(t, err, test.ShouldBeNil)

	start := referenceframe.FrameSystemInputs{"ur5e" : {{1.0471667423817}, {0.011108350341463286}, {-1.0899013011625651}, {-3.0938870331059594}, {-1.767558957758243e-05}, {-3.681258507284093}}}

	t.ResetTimer()

	for range t.N {
		_, err = c.findSeed(
			referenceframe.FrameSystemPoses{"ur5e": referenceframe.NewPoseInFrame("world",
				spatialmath.NewPose(
					r3.Vector{X:-337.976430, Y:-464.051182, Z:554.695381},
					&spatialmath.OrientationVectorDegrees{OX:0.499987, OY:-0.866033, OZ:-0.000000, Theta:0.000000},
				))},
			start, logger)
		test.That(t, err, test.ShouldBeNil)
	}

}
