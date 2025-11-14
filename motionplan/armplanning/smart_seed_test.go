//go:build !386

package armplanning

import (
	"context"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var pirIdealJointValues = [][]referenceframe.Input{
	{0 * 3.1415 / 180.0, 0, -90 * 3.1415 / 180.0, 0, 0, 0},
	{30 * 3.1415 / 180.0, 0, -90 * 3.1415 / 180.0, 0, 0, 0},
	{60 * 3.1415 / 180.0, 0, -90 * 3.1415 / 180.0, 0, 0, 0},
	{90 * 3.1415 / 180.0, 0, -90 * 3.1415 / 180.0, 0, 0, 0},
	{120 * 3.1415 / 180.0, 0, -90 * 3.1415 / 180.0, 0, 0, 0},
	{150 * 3.1415 / 180.0, 0, -90 * 3.1415 / 180.0, 0, 0, 0},
	{180 * 3.1415 / 180.0, 0, -90 * 3.1415 / 180.0, 0, 0, 0},
	{180 * 3.1415 / 180.0, 0, -90 * 3.1415 / 180.0, 0, 0, 0},
	{150 * 3.1415 / 180.0, 0, -90 * 3.1415 / 180.0, 0, 0, 0},
	{120 * 3.1415 / 180.0, 0, -90 * 3.1415 / 180.0, 0, 0, 0},
	{90 * 3.1415 / 180.0, 0, -90 * 3.1415 / 180.0, 0, 0, 0},
	{60 * 3.1415 / 180.0, 0, -90 * 3.1415 / 180.0, 0, 0, 0},
	{30 * 3.1415 / 180.0, 0, -90 * 3.1415 / 180.0, 0, 0, 0},
	{0 * 3.1415 / 180.0, 0, -90 * 3.1415 / 180.0, 0, 0, 0},
}

func TestSmartSeedCache1(t *testing.T) {
	if Is32Bit() {
		t.Skip()
		return
	}
	ctx := context.Background()
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
		1.0471667423817,
		0.011108350341463286,
		-1.0899013011625651,
		-3.0938870331059594,
		-1.767558957758243e-05,
		-3.681258507284093,
	}}.ToLinearInputs()

	goal := spatialmath.NewPose(
		r3.Vector{X: -337.976430, Y: -464.051182, Z: 554.695381},
		&spatialmath.OrientationVectorDegrees{OX: 0.499987, OY: -0.866033, OZ: -0.000000, Theta: 0.000000},
	)

	t.Run("partial", func(t *testing.T) {
		startTime := time.Now()
		seeds, _, err := c.findSeedsForFrame(
			"ur5e",
			start.Get("ur5e"),
			goal,
			1,
			logger)
		logger.Infof("time to run findSeedsForFrame: %v", time.Since(startTime))
		test.That(t, err, test.ShouldBeNil)
		cost := myCost(start.Get("ur5e"), seeds[0])
		test.That(t, cost, test.ShouldBeLessThan, .6)
	})

	t.Run("real", func(t *testing.T) {
		startTime := time.Now()
		seeds, _, err := c.findSeeds(ctx,
			referenceframe.FrameSystemPoses{"ur5e": referenceframe.NewPoseInFrame("world", goal)},
			start,
			10,
			logger)
		test.That(t, err, test.ShouldBeNil)
		logger.Infof("time to run findSeed: %v", time.Since(startTime))
		best := 100000.0
		for _, s := range seeds {
			cost := myCost(s.Get("ur5e"), s.Get("ur5e"))
			best = min(best, cost)
		}
		test.That(t, best, test.ShouldBeLessThan, .6)
	})
}

func TestSmartSeedCacheFrames(t *testing.T) {
	if Is32Bit() {
		t.Skip()
		return
	}

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

		f, p, err := c.findMovingInfo(
			referenceframe.FrameSystemInputs{
				"arm": []referenceframe.Input{0, 0, 0, 0, 0, 0},
			}.ToLinearInputs(),
			"gripper",
			referenceframe.NewPoseInFrame("world", spatialmath.NewPose(r3.Vector{}, &spatialmath.OrientationVectorDegrees{})),
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, f, test.ShouldEqual, "arm")
		test.That(t, p.Point().X, test.ShouldAlmostEqual, 10)
		test.That(t, p.Point().Y, test.ShouldAlmostEqual, -10)
		test.That(t, p.Point().Z, test.ShouldAlmostEqual, 10)
	})
}

func TestSmartSeedCachePirouette(t *testing.T) {
	ctx := context.Background()
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
		seeds, _, err := ssc.findSeeds(ctx,
			referenceframe.FrameSystemPoses{armName: referenceframe.NewPoseInFrame("world", pose)},
			referenceframe.FrameSystemInputs{armName: idealJointValues[0]}.ToLinearInputs(),
			10,
			logger)
		test.That(t, err, test.ShouldBeNil)
		firstScore := 0.0
		for ii, seed := range seeds {
			score2 := referenceframe.InputsL2Distance(seed.Get(armName), idealJointValues[i])
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
		}

		if score1 > 0 {
			test.That(t, firstScore, test.ShouldBeLessThan, 5)
		}
	}
}

func BenchmarkSmartSeedCacheSearch(t *testing.B) {
	ctx := context.Background()
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
		1.0471667423817,
		0.011108350341463286,
		-1.0899013011625651,
		-3.0938870331059594,
		-1.767558957758243e-05,
		-3.681258507284093,
	}}.ToLinearInputs()

	t.ResetTimer()

	for range t.N {
		_, err = c.findSeed(ctx,
			referenceframe.FrameSystemPoses{"ur5e": referenceframe.NewPoseInFrame("world",
				spatialmath.NewPose(
					r3.Vector{X: -337.976430, Y: -464.051182, Z: 554.695381},
					&spatialmath.OrientationVectorDegrees{OX: 0.499987, OY: -0.866033, OZ: -0.000000, Theta: 0.000000},
				))},
			start, logger)
		test.That(t, err, test.ShouldBeNil)
	}
}
