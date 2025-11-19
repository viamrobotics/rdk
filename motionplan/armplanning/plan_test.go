package armplanning

import (
	"fmt"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestEvaluateTrajectory(t *testing.T) {
	plan := motionplan.Trajectory{
		referenceframe.FrameSystemInputs{"": {1., 2., 3.}},
		referenceframe.FrameSystemInputs{"": {1., 2., 3.}},
	}
	// Test no change
	score := plan.EvaluateCost(motionplan.FSConfigurationL2Distance)
	test.That(t, score, test.ShouldAlmostEqual, 0)

	// Test L2 for "", and nothing for plan with only one entry
	plan = append(plan, referenceframe.FrameSystemInputs{"": {4., 5., 6.}, "test": {2., 3., 4.}})
	score = plan.EvaluateCost(motionplan.FSConfigurationL2Distance)
	test.That(t, score, test.ShouldAlmostEqual, math.Sqrt(27))

	// Test cumulative L2 after returning to original inputs
	plan = append(plan, referenceframe.FrameSystemInputs{"": {1., 2., 3.}})
	score = plan.EvaluateCost(motionplan.FSConfigurationL2Distance)
	test.That(t, score, test.ShouldAlmostEqual, math.Sqrt(27)*2)

	// Test that the "test" inputs are properly evaluated after skipping a step
	plan = append(plan, referenceframe.FrameSystemInputs{"test": {3., 5., 6.}})
	score = plan.EvaluateCost(motionplan.FSConfigurationL2Distance)
	test.That(t, score, test.ShouldAlmostEqual, math.Sqrt(27)*2+3)
}

func TestPlanStep(t *testing.T) {
	baseNameA := "my-base1"
	baseNameB := "my-base2"
	poseA := spatialmath.NewZeroPose()
	poseB := spatialmath.NewPose(r3.Vector{X: 100}, spatialmath.NewOrientationVector())

	protoAB := &pb.PlanStep{
		Step: map[string]*pb.ComponentState{
			baseNameA: {Pose: spatialmath.PoseToProtobuf(poseA)},
			baseNameB: {Pose: spatialmath.PoseToProtobuf(poseB)},
		},
	}
	stepAB := referenceframe.FrameSystemPoses{
		baseNameA: referenceframe.NewPoseInFrame(referenceframe.World, poseA),
		baseNameB: referenceframe.NewPoseInFrame(referenceframe.World, poseB),
	}

	t.Run("FrameSystemPosesFromProto", func(t *testing.T) {
		type testCase struct {
			description string
			input       *pb.PlanStep
			result      referenceframe.FrameSystemPoses
			err         error
		}

		testCases := []testCase{
			{
				description: "nil pointer returns an error",
				input:       nil,
				result:      referenceframe.FrameSystemPoses{},
				err:         errors.New("received nil *pb.PlanStep"),
			},
			{
				description: "an empty *pb.PlanStep returns an empty referenceframe.FrameSystemPoses{}",
				input:       &pb.PlanStep{},
				result:      referenceframe.FrameSystemPoses{},
			},
			{
				description: "a full *pb.PlanStep returns an full referenceframe.FrameSystemPoses{}",
				input:       protoAB,
				result:      stepAB,
			},
		}
		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res, err := motionplan.FrameSystemPosesFromProto(tc.input)
				if tc.err != nil {
					test.That(t, err, test.ShouldBeError, tc.err)
				} else {
					test.That(t, err, test.ShouldBeNil)
				}
				test.That(t, res, test.ShouldResemble, tc.result)
			})
		}
	})

	t.Run("FrameSystemPosesToProto()", func(t *testing.T) {
		type testCase struct {
			description string
			input       referenceframe.FrameSystemPoses
			result      *pb.PlanStep
		}

		testCases := []testCase{
			{
				description: "an nil referenceframe.FrameSystemPoses returns an empty *pb.PlanStep",
				input:       nil,
				result:      &pb.PlanStep{},
			},
			{
				description: "an empty referenceframe.FrameSystemPoses returns an empty *pb.PlanStep",
				input:       referenceframe.FrameSystemPoses{},
				result:      &pb.PlanStep{},
			},
			{
				description: "a full referenceframe.FrameSystemPoses{} returns an full *pb.PlanStep",
				input:       stepAB,
				result:      protoAB,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res := motionplan.FrameSystemPosesToProto(tc.input)
				test.That(t, res, test.ShouldResemble, tc.result)
			})
		}
	})
}

// BenchmarkGoalMetric
//
// Old:
// - BenchmarkGoalMetric-16                 	  532743	      3448 ns/op	    2416 B/op	      42 allocs/op
// New:
// - BenchmarkGoalMetric-16                 	  801975	      1247 ns/op	     800 B/op	      14 allocs/op
// - BenchmarkGoalMetric-16                 	  940275	      1145 ns/op	     640 B/op	      11 allocs/op
// - BenchmarkGoalMetric-24                 	  312554	      3740 ns/op	     336 B/op	       6 allocs/op
// - BenchmarkGoalMetric-16                    	 1214918         980.4 ns/op	     208 B/op	       4 allocs/op
func BenchmarkGoalMetric(b *testing.B) {
	goalInFrame := referenceframe.NewPoseInFrame(
		"world",
		&spatialmath.DualQuaternion{
			Number: dualquat.Number{
				Real: quat.Number{
					Real: 0.0051161120465661614, Imag: 0, Jmag: 0.9999869126131237, Kmag: 0,
				},
				Dual: quat.Number{
					Real: -798.2045339836992, Imag: 241.2609122119685, Jmag: 4.083757277649134, Kmag: -822.820958100296,
				},
			},
		},
	)
	goalInFrame.SetName("xarm6")

	options := &PlannerOptions{
		GoalMetricType: motionplan.SquaredNorm,
	}

	armModel, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "xarm6")
	test.That(b, err, test.ShouldBeNil)

	// Create a temporary frame system for the transformation
	fs := referenceframe.NewEmptyFrameSystem("")
	err = fs.AddFrame(armModel, fs.World())
	test.That(b, err, test.ShouldBeNil)

	metricFn := options.getGoalMetric(referenceframe.FrameSystemPoses{"xarm6": goalInFrame})
	test.That(b, err, test.ShouldBeNil)

	inps := referenceframe.NewLinearInputs()
	inps.Put("xarm6", []referenceframe.Input{
		-1.335, -1.334, -1.339, -1.338, -1.337, -1.336,
	})

	// TODO: create a single `StateFS` object for all calls?
	ans := metricFn(&motionplan.StateFS{
		Configuration: inps,
		FS:            fs,
	})
	test.That(b, ans, test.ShouldAlmostEqual, 6.1075976675485745e+06)

	for b.Loop() {
		metricFn(&motionplan.StateFS{
			Configuration: inps,
			FS:            fs,
		})
	}
}

// Not exactly an `armplanning` benchmark, but it's an important part of calling the scoring
// metric. This can help isolate if scoring is slow because armplan scores are slow, or if its
// because framesystem transformations are slow.
func BenchmarkFSTransform(b *testing.B) {
	armModel, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "xarm6")
	test.That(b, err, test.ShouldBeNil)

	// Create a temporary frame system for the transformation
	fs := referenceframe.NewEmptyFrameSystem("")
	err = fs.AddFrame(armModel, fs.World())
	test.That(b, err, test.ShouldBeNil)

	inputs := referenceframe.NewLinearInputs()
	inputs.Put("xarm6", []referenceframe.Input{
		-1.335, -1.334, -1.339, -1.338, -1.337, -1.336,
	})

	outputPose, err := fs.Transform(inputs, referenceframe.NewZeroPoseInFrame("xarm6"), "world")
	test.That(b, err, test.ShouldBeNil)
	test.That(b, fmt.Sprintf("%v", outputPose), test.ShouldEqual,
		"parent: world, pose: {X:53.425180 Y:243.992738 Z:692.082423 OX:0.898026 OY:0.314087 OZ:0.308055 Theta:130.963386°}")

	accumulator := referenceframe.NewZeroPoseInFrame("xarm6")
	for b.Loop() {
		fs.Transform(inputs, accumulator, "world")
	}
}

// BenchmarkScaledSquaredNormMetric measures the distance evaluation between the goal state and
// the state computed from the inputs. This is the same as the above `BenchmarkGoalMetric`,
// except the above is also measuring the computational part of arriving at the sampled pose.
//
// Old:
// - BenchmarkScaledSquaredNormMetric-16    	 6429082	       205.4 ns/op	      64 B/op	       1 allocs/op
//
// New:
// - BenchmarkScaledSquaredNormMetric-16    	 8371084	       148.3 ns/op	       0 B/op	       0 allocs/op
func BenchmarkScaledSquaredNormMetric(b *testing.B) {
	goalFrame := referenceframe.NewPoseInFrame(
		"world",
		&spatialmath.DualQuaternion{
			Number: dualquat.Number{
				Real: quat.Number{
					Real: 0.0051161120465661614, Imag: 0, Jmag: 0.9999869126131237, Kmag: 0,
				},
				Dual: quat.Number{
					Real: -798.2045339836992, Imag: 241.2609122119685, Jmag: 4.083757277649134, Kmag: -822.820958100296,
				},
			},
		},
	)
	goalFrame.SetName("xarm6")

	for b.Loop() {
		motionplan.WeightedSquaredNormDistance(goalFrame.Pose(), spatialmath.NewZeroPose())
	}
}

// This optimization inclues:
// - Don't allocate array when not capturing all sub-arm pose information.
// - Call optimized version of `*staticFrame.Transform`.
// - Call optimized version of `*rotationalFrame.Transform`.
//
// Old:
// - BenchmarkArmTransform-16               	  554766	      2817 ns/op	    1680 B/op	      29 allocs/op
// New:
// Avoid array slicing when frame input size is 0 (static) or 1 (rotational).
// - BenchmarkArmTransform-16            	  563020	      2060 ns/op	    1472 B/op	      26 allocs/op
// One unroll of `*rotationalFrame.Transform`
// - BenchmarkArmTransform-16            	 1000000	      1880 ns/op	    1280 B/op	      20 allocs/op
// Inline `composedTransformation`
// - BenchmarkArmTransform-16            	 1830990	       653.5 ns/op	      64 B/op	       1 allocs/op
func BenchmarkArmTransform(b *testing.B) {
	armModelI, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "xarm6")
	test.That(b, err, test.ShouldBeNil)
	armModel := armModelI.(*referenceframe.SimpleModel)

	inps := []referenceframe.Input{
		-1.335, -1.334, -1.339, -1.338, -1.337, -1.336,
	}

	// Not useful if we don't get the right answer.
	pose, err := armModel.Transform(inps)
	test.That(b, err, test.ShouldBeNil)
	test.That(b, fmt.Sprintf("%v", pose), test.ShouldEqual,
		"{X:53.425180 Y:243.992738 Z:692.082423 OX:0.898026 OY:0.314087 OZ:0.308055 Theta:130.963386°}")

	for b.Loop() {
		armModel.Transform(inps)
	}
}

// Old:
// - BenchmarkLinearizeFSMetric-16          	 2150169	       531.7 ns/op	     864 B/op	      11 allocs/op
// Old + No Spans:
// - BenchmarkLinearizeFSMetric-16          	 7035686	       174.6 ns/op	     448 B/op	       3 allocs/op
// New:
// - BenchmarkLinearizeFSMetric-16          	 3936822	       305.3 ns/op	     416 B/op	       8 allocs/op
// New + No Spans:
// - BenchmarkLinearizeFSMetric-16          	556656800	       2.154 ns/op	       0 B/op	       0 allocs/op
func BenchmarkLinearizeFSMetric(b *testing.B) {
	ctx := b.Context()
	logger := logging.NewTestLogger(b)

	armModel, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "xarm6")
	test.That(b, err, test.ShouldBeNil)

	// Create a temporary frame system for the transformation
	fs := referenceframe.NewEmptyFrameSystem("")
	err = fs.AddFrame(armModel, fs.World())
	test.That(b, err, test.ShouldBeNil)

	pc, err := newPlanContext(ctx, logger,
		&PlanRequest{
			FrameSystem:    fs,
			PlannerOptions: &PlannerOptions{},
		},
		&PlanMeta{},
	)
	test.That(b, err, test.ShouldBeNil)

	inps := []float64{
		-1.335, -1.334, -1.339, -1.338, -1.337, -1.336,
	}

	fsInps, err := pc.lis.FloatsToInputs(inps)
	test.That(b, err, test.ShouldBeNil)

	// Not useful if the code gets the wrong answer.
	test.That(b, fsInps.ToFrameSystemInputs(), test.ShouldResemble, referenceframe.FrameSystemInputs(
		map[string][]referenceframe.Input{
			"xarm6": {
				-1.335, -1.334, -1.339, -1.338, -1.337, -1.336,
			},
		},
	))

	minFunc := pc.linearizeFSmetric(func(_ *motionplan.StateFS) float64 {
		return 0.0
	})

	for b.Loop() {
		minFunc(ctx, inps)
	}
}
