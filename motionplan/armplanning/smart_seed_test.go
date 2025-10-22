package armplanning

import (
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestSmartSeedCache1(t *testing.T) {
	logger := logging.NewTestLogger(t)

	armName := "ur5e"
	armKinematics, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), armName)
	test.That(t, err, test.ShouldBeNil)

	fs := referenceframe.NewEmptyFrameSystem("pirouette")
	err = fs.AddFrame(armKinematics, fs.World())
	test.That(t, err, test.ShouldBeNil)

	c, err := smartSeed(fs, logger)
	test.That(t, err, test.ShouldBeNil)

	start := referenceframe.FrameSystemInputs{"ur5e": {
		{1.0471667423817},
		{0.011108350341463286},
		{-1.0899013011625651},
		{-3.0938870331059594},
		{-1.767558957758243e-05},
		{-3.681258507284093},
	}}

	goal := spatialmath.NewPose(
		r3.Vector{X: -337.976430, Y: -464.051182, Z: 554.695381},
		&spatialmath.OrientationVectorDegrees{OX: 0.499987, OY: -0.866033, OZ: -0.000000, Theta: 0.000000},
	)

	t.Run("partial", func(t *testing.T) {
		startTime := time.Now()
		seeds, err := c.findSeedsForFrame(
			"ur5e",
			start["ur5e"],
			goal,
			logger)
		test.That(t, err, test.ShouldBeNil)
		logger.Infof("time to run findSeedsForFrame: %v", time.Since(startTime))

		cost := referenceframe.InputsL2Distance(start["ur5e"], seeds[0])
		test.That(t, cost, test.ShouldBeLessThan, 1.25)
	})

	t.Run("real", func(t *testing.T) {
		startTime := time.Now()
		seed, err := c.findSeed(
			referenceframe.FrameSystemPoses{"ur5e": referenceframe.NewPoseInFrame("world", goal)},
			start,
			logger)
		test.That(t, err, test.ShouldBeNil)
		logger.Infof("time to run findSeed: %v", time.Since(startTime))

		cost := referenceframe.InputsL2Distance(start["ur5e"], seed["ur5e"])
		test.That(t, cost, test.ShouldBeLessThan, 1.25)
	})
}

func TestSmartSeedCacheFrames(t *testing.T) {
	logger := logging.NewTestLogger(t)

	armName := "arm"
	armKinematics, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), armName)
	test.That(t, err, test.ShouldBeNil)

	gripperFrame, err := referenceframe.NewStaticFrame("gripper", spatialmath.NewPoseFromPoint(r3.Vector{10, 10, 10}))
	test.That(t, err, test.ShouldBeNil)

	t.Run("gripper", func(t *testing.T) {
		fs := referenceframe.NewEmptyFrameSystem("pirouette")

		err = fs.AddFrame(armKinematics, fs.World())
		test.That(t, err, test.ShouldBeNil)
		err = fs.AddFrame(gripperFrame, fs.Frame("arm"))
		test.That(t, err, test.ShouldBeNil)

		c, err := smartSeed(fs, logger)
		test.That(t, err, test.ShouldBeNil)

		f, p, err := c.findMovingInfo(referenceframe.FrameSystemInputs{
			"arm": []referenceframe.Input{{0}, {0}, {0}, {0}, {0}, {0}},
		},
			"gripper", referenceframe.NewPoseInFrame("world", spatialmath.NewPose(r3.Vector{}, &spatialmath.OrientationVectorDegrees{})))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, f, test.ShouldEqual, "arm")
		test.That(t, p.Point(), test.ShouldResemble, r3.Vector{100, 100, 100})
	})
}

func TestSmartSeedCachePirouette(t *testing.T) {
	logger := logging.NewTestLogger(t)

	armName := "ur5e"
	armKinematics, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), armName)
	test.That(t, err, test.ShouldBeNil)

	idealJointValues := pirIdealJointValues

	fs := referenceframe.NewEmptyFrameSystem("pirouette")
	err = fs.AddFrame(armKinematics, fs.World())
	test.That(t, err, test.ShouldBeNil)

	ssc, err := smartSeed(fs, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)

	for i, ideal := range idealJointValues {
		pose, err := armKinematics.Transform(ideal)
		test.That(t, err, test.ShouldBeNil)

		score1 := referenceframe.InputsL2Distance(idealJointValues[0], ideal)
		logger.Infof("hi %d %v", i, score1)
		seeds, err := ssc.findSeeds(
			referenceframe.FrameSystemPoses{armName: referenceframe.NewPoseInFrame("world", pose)},
			referenceframe.FrameSystemInputs{armName: idealJointValues[0]},
			logger)
		test.That(t, err, test.ShouldBeNil)
		firstScore := 0.0
		for ii, seed := range seeds {
			score2 := referenceframe.InputsL2Distance(seed[armName], idealJointValues[i])
			if ii == 0 {
				firstScore = score2
			}
			logger.Infof("\t %d %v", ii, score2)
			if score2 < score1 {
				break
			}
			if score1 == 0 {
				break
			}
			if ii > 10 {
				break
			}
		}

		if score1 > 0 {
			test.That(t, firstScore, test.ShouldBeLessThan, 4)
		}
	}
}

func BenchmarkSmartSeedCacheSearch(t *testing.B) {
	logger := logging.NewTestLogger(t)

	armName := "ur5e"
	armKinematics, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), armName)
	test.That(t, err, test.ShouldBeNil)

	fs := referenceframe.NewEmptyFrameSystem("pirouette")
	err = fs.AddFrame(armKinematics, fs.World())
	test.That(t, err, test.ShouldBeNil)

	c, err := smartSeed(fs, logger)
	test.That(t, err, test.ShouldBeNil)

	start := referenceframe.FrameSystemInputs{"ur5e": {
		{1.0471667423817},
		{0.011108350341463286},
		{-1.0899013011625651},
		{-3.0938870331059594},
		{-1.767558957758243e-05},
		{-3.681258507284093},
	}}

	t.ResetTimer()

	for range t.N {
		_, err = c.findSeed(
			referenceframe.FrameSystemPoses{"ur5e": referenceframe.NewPoseInFrame("world",
				spatialmath.NewPose(
					r3.Vector{X: -337.976430, Y: -464.051182, Z: 554.695381},
					&spatialmath.OrientationVectorDegrees{OX: 0.499987, OY: -0.866033, OZ: -0.000000, Theta: 0.000000},
				))},
			start, logger)
		test.That(t, err, test.ShouldBeNil)
	}
}
