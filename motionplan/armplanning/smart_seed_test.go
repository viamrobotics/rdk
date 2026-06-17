//go:build !386

package armplanning

import (
	"context"
	"math"
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
	if IsTooSmallForCache() {
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
			10,
			logger)
		logger.Infof("time to run findSeedsForFrame: %v", time.Since(startTime))
		test.That(t, err, test.ShouldBeNil)
		best := 100000.0
		for _, s := range seeds {
			cost := myCost(start.Get("ur5e"), s)
			best = min(best, cost)
		}
		logger.Infof("best: %v\n", best)
		test.That(t, best, test.ShouldBeLessThan, 1.5)
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
			cost := myCost(start.Get("ur5e"), s.Get("ur5e"))
			best = min(best, cost)
		}
		logger.Infof("best: %v\n", best)
		test.That(t, best, test.ShouldBeLessThan, 1.5)
	})
}

func TestSmartSeedCacheFrames(t *testing.T) {
	if IsTooSmallForCache() {
		t.Skip()
		return
	}

	logger := logging.NewTestLogger(t)

	armName := "arm"
	armKinematics, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), armName)
	test.That(t, err, test.ShouldBeNil)

	gripperFrame, err := referenceframe.NewStaticFrame("gripper", spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 100}))
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
				"arm": []referenceframe.Input{0, 1, 0, 1, 0, 1},
			}.ToLinearInputs(),
			"gripper",
			referenceframe.NewPoseInFrame("world", spatialmath.NewPose(r3.Vector{}, &spatialmath.OrientationVectorDegrees{OX: 1})),
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, f, test.ShouldEqual, "arm")
		test.That(t, p.Point().X, test.ShouldAlmostEqual, -100)
		test.That(t, p.Point().Y, test.ShouldAlmostEqual, 0)
		test.That(t, p.Point().Z, test.ShouldAlmostEqual, 0)
	})
}

// TestSmartSeedCacheOffsetMount verifies that findSeeds works when the arm is mounted
// far from the world origin. Without the parent-coordinate-system fix in findMovingInfo,
// the goal's norm in world coordinates (~2800mm) exceeds the cache's maxNorm (~1017mm)
// and triggers a false "too far" error.
func TestSmartSeedCacheOffsetMount(t *testing.T) {
	if IsTooSmallForCache() {
		t.Skip()
		return
	}
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	armName := "ur5e"
	armKinematics, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), armName)
	test.That(t, err, test.ShouldBeNil)

	// Mount the arm 2000mm from world origin via a static offset frame.
	mountPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 2000, Y: 0, Z: 0})
	mount, err := referenceframe.NewStaticFrame("mount", mountPose)
	test.That(t, err, test.ShouldBeNil)

	fs := referenceframe.NewEmptyFrameSystem("offset_test")
	err = fs.AddFrame(mount, fs.World())
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(armKinematics, fs.Frame("mount"))
	test.That(t, err, test.ShouldBeNil)

	ssc, err := smartSeed(fs, logger)
	test.That(t, err, test.ShouldBeNil)

	startJoints := []referenceframe.Input{
		1.0471667423817,
		0.011108350341463286,
		-1.0899013011625651,
		-3.0938870331059594,
		-1.767558957758243e-05,
		-3.681258507284093,
	}
	start := referenceframe.FrameSystemInputs{armName: startJoints}.ToLinearInputs()

	// Compute a goal pose from a different joint configuration so that the goal is
	// reachable but distinct from the start (the seed finder filters out seeds that
	// are further than the start).
	goalJoints := []referenceframe.Input{0.52, 0, -1.57, 0, 0, 0}
	localPose, err := armKinematics.Transform(goalJoints)
	test.That(t, err, test.ShouldBeNil)
	goalInWorld := spatialmath.Compose(mountPose, localPose)

	// Sanity check: the goal norm in world coords must exceed the cache maxNorm
	// (~1017mm for the ur5e). This is the condition that triggers the false
	// "too far" error without the parent-coordinate-system fix.
	goalNorm := goalInWorld.Point().Norm()
	test.That(t, goalNorm, test.ShouldBeGreaterThan, ssc.rawCache[armName].maxNorm)
	logger.Infof("goal norm in world: %0.2f, cache maxNorm: %0.2f", goalNorm, ssc.rawCache[armName].maxNorm)

	// This call would fail with tooFarError if findMovingInfo doesn't transform the
	// goal into the frame's parent coordinate system.
	seeds, _, err := ssc.findSeeds(ctx,
		referenceframe.FrameSystemPoses{armName: referenceframe.NewPoseInFrame("world", goalInWorld)},
		start, 10, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(seeds), test.ShouldBeGreaterThan, 0)

	best := 100000.0
	for _, s := range seeds {
		cost := myCost(start.Get(armName), s.Get(armName))
		best = min(best, cost)
	}
	logger.Infof("best seed cost: %v", best)
	test.That(t, best, test.ShouldBeLessThan, 1.5)
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

// TestSmartSeedPallette2 exercises smartSeed.findSeeds against the goal+start
// from data/pallette2.json and asserts that the seeds it returns are useful:
//
//  1. at least one seed is returned,
//  2. the best seed's end-effector is closer to the goal than the start is,
//  3. the best seed's joint-space cost from start is reasonable.
//
// Each assertion is independent so a failure tells us which property of the
// smart-seed output went wrong; the surrounding log lines give the raw
// distances, costs, and per-seed FK points that explain why.
func TestSmartSeedPallette2(t *testing.T) {
	t.Parallel()
	if IsTooSmallForCache() {
		t.Skip()
		return
	}

	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	req, err := ReadRequestFromFile("data/pallette2.json")
	test.That(t, err, test.ShouldBeNil)

	pc, err := NewPlanContext(ctx, logger, req, &PlanMeta{})
	test.That(t, err, test.ShouldBeNil)

	psc, err := NewPlanSegmentContext(ctx, pc, req.StartState.LinearConfiguration(), req.Goals[0].Poses())
	test.That(t, err, test.ShouldBeNil)

	// Resolve which frame is actually moving toward the goal, and where the goal
	// lives in that frame's mounting parent — same logic findSeeds uses internally.
	var goalFrame string
	var goalPIF *referenceframe.PoseInFrame
	for k, v := range psc.goal {
		goalFrame = k
		goalPIF = v
	}
	ssc, err := smartSeed(pc.fs, logger)
	test.That(t, err, test.ShouldBeNil)

	movingFrameName, movingPose, err := ssc.findMovingInfo(psc.start, goalFrame, goalPIF)
	test.That(t, err, test.ShouldBeNil)
	logger.Infof("moving frame: %s, goal in mount frame: %v", movingFrameName, movingPose)

	frame := pc.fs.Frame(movingFrameName)
	startInputs := psc.start.Get(movingFrameName)
	startPose, err := frame.Transform(startInputs)
	test.That(t, err, test.ShouldBeNil)
	startFKDist := startPose.Point().Distance(movingPose.Point())
	logger.Infof("start FK distance to goal: %0.2f (start joints: %v)",
		startFKDist, logging.FloatArrayFormat{"%0.2f", startInputs})

	seeds, divisors, err := ssc.findSeeds(ctx, psc.goal, psc.start, 10, logger)
	test.That(t, err, test.ShouldBeNil)
	logger.Infof("got %d seeds, divisors=%v", len(seeds), divisors)

	// 1. We expect at least one seed.
	test.That(t, len(seeds), test.ShouldBeGreaterThan, 0)

	// Known-good joint configuration for the goal — at least one seed should
	// land near this in joint space, otherwise the seed pool won't lead IK to
	// the right basin.
	target := []referenceframe.Input{2.23, -1.59, 1.51, -1.49, -1.57, 0.66}

	bestCost := math.Inf(1)
	bestFKDist := math.Inf(1)
	bestTargetL2 := math.Inf(1)
	for i, s := range seeds {
		sInputs := s.Get(movingFrameName)
		cost := myCost(startInputs, sInputs)
		pose, err := frame.Transform(sInputs)
		test.That(t, err, test.ShouldBeNil)
		fkDist := pose.Point().Distance(movingPose.Point())
		targetL2 := referenceframe.InputsL2Distance(sInputs, target)
		logger.Infof("seed %2d: cost=%0.2f fkDist=%0.2f targetL2=%0.2f joints=%v",
			i, cost, fkDist, targetL2, logging.FloatArrayFormat{"%0.2f", sInputs})
		if cost < bestCost {
			bestCost = cost
		}
		if fkDist < bestFKDist {
			bestFKDist = fkDist
		}
		if targetL2 < bestTargetL2 {
			bestTargetL2 = targetL2
		}
	}
	logger.Infof("summary: bestCost=%0.2f bestFKDist=%0.2f bestTargetL2=%0.2f (startFKDist=%0.2f)",
		bestCost, bestFKDist, bestTargetL2, startFKDist)

	// 2. The best seed has to actually pull the end-effector closer to the goal,
	//    otherwise the seed buys us nothing over re-running IK from start.
	test.That(t, bestFKDist, test.ShouldBeLessThan, startFKDist)

	// 3. The best seed shouldn't require massive joint reconfiguration.
	test.That(t, bestCost, test.ShouldBeLessThan, 3.0)

	// 4. At least one seed should land in joint-space neighborhood of the known
	//    correct goal configuration — otherwise IK started from these seeds is
	//    unlikely to converge to the right basin.
	test.That(t, bestTargetL2, test.ShouldBeLessThan, 3.0)

	// 5. The IK search window built around at least one seed must actually
	//    contain the target. ComputeAdjustLimitsArray clamps each joint to
	//    [seed-r*divisor, seed+r*divisor] (intersected with the global joint
	//    limits) — if the target falls outside every seed's window, IK can
	//    never reach it no matter how good the convergence.
	divInputs, err := pc.lis.FloatsToInputs(divisors)
	test.That(t, err, test.ShouldBeNil)
	movingDivisors := divInputs.Get(movingFrameName)
	movingLimits := frame.DoF()

	anyContains := false
	for i, s := range seeds {
		sInputs := s.Get(movingFrameName)
		contains := true
		for j, lim := range movingLimits {
			_, _, r := lim.GoodLimits()
			d := r * movingDivisors[j]
			lo := math.Max(lim.Min, sInputs[j]-d)
			hi := math.Min(lim.Max, sInputs[j]+d)
			if target[j] < lo || target[j] > hi {
				logger.Infof("seed %2d: joint %d target=%0.2f outside window [%0.2f,%0.2f] (seed=%0.2f, d=%0.2f)",
					i, j, target[j], lo, hi, sInputs[j], d)
				contains = false
				break
			}
		}
		if contains {
			logger.Infof("seed %2d: window contains target", i)
			anyContains = true
		}
	}
	test.That(t, anyContains, test.ShouldBeTrue)
}
