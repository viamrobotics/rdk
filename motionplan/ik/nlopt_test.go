package ik

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/kr/pretty"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"
	"golang.org/x/exp/slices"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestCreateNloptSolver(t *testing.T) {
	logger := logging.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// matches xarm home end effector position
	pos := spatialmath.NewPoseFromPoint(r3.Vector{X: 207, Z: 112})
	seed := []float64{1, 1, -1, 1, 1, 0}
	solveFunc := NewMetricMinFunc(motionplan.NewScaledSquaredNormMetric(pos, 10), m, logger, motionplan.TranslationCloud{})

	t.Run("not exact", func(t *testing.T) {
		ik, err := CreateNloptSolver(logger, -1, false, true, time.Second)
		test.That(t, err, test.ShouldBeNil)

		_, _, err = DoSolve(context.Background(), ik, solveFunc, [][]float64{seed}, [][]referenceframe.Limit{m.DoF()})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("exact", func(t *testing.T) {
		ik, err := CreateNloptSolver(logger, -1, true, true, time.Second)
		test.That(t, err, test.ShouldBeNil)

		_, meta, err := DoSolve(context.Background(), ik, solveFunc, [][]float64{seed}, [][]referenceframe.Limit{m.DoF()})
		test.That(t, err, test.ShouldBeNil)
		for idx, m := range meta {
			logger.Debugf("seed: %d %#v", idx, m)
		}
	})

	t.Run("Check unpacking from proto", func(t *testing.T) {
		pos = spatialmath.NewPose(
			r3.Vector{X: -46, Y: -23, Z: 372},
			&spatialmath.OrientationVectorDegrees{Theta: 0, OX: 0, OY: 0, OZ: -1},
		)

		seed = m.InputFromProtobuf(&pb.JointPositions{Values: []float64{49, 28, -101, 0, -73, 0}})
		solveFunc = NewMetricMinFunc(motionplan.NewSquaredNormMetric(pos), m, logger)

		ik, err := CreateNloptSolver(logger, -1, false, true, time.Second)
		test.That(t, err, test.ShouldBeNil)

		_, _, err = DoSolve(context.Background(), ik, solveFunc, [][]float64{seed}, [][]referenceframe.Limit{m.DoF()})
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestSolveMinFuncs(t *testing.T) {
	logger := logging.NewTestLogger(t).Sublogger("mp.ik")
	var armFrame referenceframe.Frame

	armFrame, err := referenceframe.ParseModelJSONFile(
		utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	nloptSolver, err := CreateNloptSolver(logger, -1, true, true, time.Second)
	test.That(t, err, test.ShouldBeNil)

	goalPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 207, Z: 112})
	var minFunc CostFunc
	if true {
		goalLeeway := r3.Vector{10, 10, 5}
		minFunc = NewMetricMinFunc(func(candidatePose spatialmath.Pose) float64 {
			// Higher value (~1.0) -- more solutions (when not using orientation scoring. Haven't
			// tried with). Gradient descent works faster? Too high (10.0) and only get ~a dozen
			// solutions. Gradient descent overshoots answers?
			const scale = 0.1
			// ptDelta := goalPose.Point().Mul(scale).Sub(candidatePose.Point().Mul(scale)).Norm2()
			// return ptDelta

			// Solutions: 575
			// Meta: [{2008 30 575}]
			// Err: <nil>

			xDiff := scale * math.Abs(goalPose.Point().X-candidatePose.Point().X)
			yDiff := scale * math.Abs(goalPose.Point().Y-candidatePose.Point().Y)
			zDiff := scale * math.Abs(goalPose.Point().Z-candidatePose.Point().Z)

			if xDiff < goalLeeway.X {
				xDiff = 0
			}
			if yDiff < goalLeeway.Y {
				yDiff = 0
			}
			if zDiff < goalLeeway.Z {
				zDiff = 0
			}

			// orientDelta := 0.0
			// orientDelta = spatialmath.QuatToR3AA(spatialmath.OrientationBetween(
			//		startPose.Orientation(),
			//		goalPose.Orientation(),
			// ).Quaternion()).Mul(10.0).Norm2()

			return (xDiff * xDiff) + (yDiff * yDiff) + (zDiff * zDiff)
		}, armFrame, logger)
	} else {
		minFunc = NewMetricMinFunc(motionplan.NewScaledSquaredNormMetric(goalPose, 0, motionplan.TranslationCloud{}),
			armFrame, logger)
	}

	seed := []float64{1, 1, -1, 1, 1, 0}
	seed = armFrame.InputFromProtobuf(&pb.JointPositions{Values: []float64{49, 28, -101, 0, -73, 0}})
	ans, meta, err := DoSolve(t.Context(), nloptSolver, minFunc,
		[][]float64{seed}, [][]referenceframe.Limit{armFrame.DoF()})

	fmt.Println("Solutions:", len(ans))
	fmt.Println("Meta:", meta)
	fmt.Println("Err:", err)

	ans = dedup(ans)

	slices.SortFunc(ans, func(left, right []float64) int {
		for idx, lVal := range left {
			diff := lVal - right[idx]
			if math.Abs(diff) < epsilon {
				continue
			}

			if diff < 0 {
				return -1
			} else {
				return 1
			}
		}

		return 0
	})
	fmt.Println("Deduped:", len(ans))
	pretty.Println(ans)

	diffs := []float64{}
	for _, inps := range ans {
		diffs = append(diffs, minFunc(t.Context(), inps))
	}

	slices.Sort(diffs)
	pretty.Println(diffs)
}

const epsilon = 1e-5

func same(left, right []float64) bool {
	for idx, lVal := range left {
		if math.Abs(lVal-right[idx]) < epsilon {
			return true
		}
	}

	return false
}

func dedup(inps [][]float64) [][]float64 {
	out := [][]float64{}
	for _, inp := range inps {
		matched := false
		for _, cmp := range out {
			if same(inp, cmp) {
				matched = true
				break
			}
		}

		if !matched {
			out = append(out, inp)
		}
	}

	return out
}
