//go:build !windows

package motionplan

import (
	"context"
	"math"
	"math/rand"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

func TestPtgRrt(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ackermanFrame, err := NewptgFrame("test")
	test.That(t, err, test.ShouldBeNil)

	goalPos := spatialmath.NewPose(r3.Vector{X: 50, Y: 10, Z: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 180})

	opt := newBasicPlannerOptions()
	opt.SetGoalMetric(NewPositionOnlyMetric(goalPos))
	opt.DistanceFunc = SquaredNormNoOrientSegmentMetric
	opt.GoalThreshold = 10.
	mp, err := newTPSpaceMotionPlanner(ackermanFrame, rand.New(rand.NewSource(42)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	tp, ok := mp.(*tpSpaceRRTMotionPlanner)
	test.That(t, ok, test.ShouldBeTrue)

	plan, err := tp.plan(context.Background(), goalPos, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan), test.ShouldBeGreaterThan, 2)
}

func TestPtgWithObstacle(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ackermanFrame, err := NewptgFrame("ackframe")
	test.That(t, err, test.ShouldBeNil)

	goalPos := spatialmath.NewPoseFromPoint(r3.Vector{X: 5000, Y: 0, Z: 0})

	fs := referenceframe.NewEmptyFrameSystem("test")
	fs.AddFrame(ackermanFrame, fs.World())

	opt := newBasicPlannerOptions()
	opt.SetGoalMetric(NewPositionOnlyMetric(goalPos))
	opt.DistanceFunc = SquaredNormNoOrientSegmentMetric
	opt.GoalThreshold = 10.
	// obstacles
	obstacle1, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2500, -500, 0}), r3.Vector{180, 1800, 1}, "")
	test.That(t, err, test.ShouldBeNil)
	obstacle2, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2500, 2000, 0}), r3.Vector{180, 1800, 1}, "")
	test.That(t, err, test.ShouldBeNil)
	obstacle3, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2500, -1400, 0}), r3.Vector{50000, 10, 1}, "")
	test.That(t, err, test.ShouldBeNil)
	obstacle4, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2500, 2400, 0}), r3.Vector{50000, 10, 1}, "")
	test.That(t, err, test.ShouldBeNil)

	geoms := []spatialmath.Geometry{obstacle1, obstacle2, obstacle3, obstacle4}

	worldState, err := referenceframe.NewWorldState(
		[]*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)
	sf, err := newSolverFrame(fs, "ackframe", referenceframe.World, nil)
	test.That(t, err, test.ShouldBeNil)
	collisionConstraints, err := createAllCollisionConstraints(sf, fs, worldState, referenceframe.StartPositions(fs), nil)
	test.That(t, err, test.ShouldBeNil)

	for name, constraint := range collisionConstraints {
		opt.AddStateConstraint(name, constraint)
	}

	mp, err := newTPSpaceMotionPlanner(ackermanFrame, rand.New(rand.NewSource(42)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	tp, _ := mp.(*tpSpaceRRTMotionPlanner)

	plan, err := tp.plan(context.Background(), goalPos, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan), test.ShouldBeGreaterThan, 2)
}

type ptgFrame struct {
	name       string
	limits     []referenceframe.Limit
	geometries []spatialmath.Geometry
	ptgs       []tpspace.PTG
}

type ptgFactory func(float64, float64, float64) tpspace.PrecomputePTG

var defaultPTGs = []ptgFactory{
	tpspace.NewCirclePTG,
	tpspace.NewCCPTG,
	tpspace.NewCCSPTG,
	tpspace.NewCSPTG,
	tpspace.NewAlphaPTG,
}

var (
	defaultMps     = 1.
	defaultSimDist = 1000.
	defaultDps     = 90.
)

// This should live elsewhere

var defaultAlphaCnt uint = 121

func NewptgFrame(name string) (referenceframe.Frame, error) {
	pf := &ptgFrame{name: name}

	ptgs, err := initPTGs(defaultMps, defaultDps)
	if err != nil {
		return nil, err
	}
	pf.ptgs = ptgs

	geometry, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{1, 1, 1}, "ackGeom")
	if err != nil {
		return nil, err
	}

	pf.geometries = []spatialmath.Geometry{geometry}

	// This is meaningless but needs to be len > 0
	pf.limits = []referenceframe.Limit{
		{Min: 0, Max: float64(len(ptgs))},
		{Min: 0, Max: float64(defaultAlphaCnt)},
		{Min: 0},
	}

	return pf, nil
}

// IMPLEMENTED FOR FRAME INTERFACE FOR TESTING
// NONE OF THIS DEFINITELY WORKS AND SHOULD BE REPLACED

func (pf *ptgFrame) DoF() []referenceframe.Limit {
	return pf.limits
}

func (pf *ptgFrame) Name() string {
	return pf.name
}

func (pf *ptgFrame) MarshalJSON() ([]byte, error) {
	return nil, nil
}

func (pf *ptgFrame) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	ptgIdx := int(math.Round(inputs[0].Value))
	trajIdx := uint(math.Round(inputs[1].Value))
	traj := pf.ptgs[ptgIdx].Trajectory(trajIdx)
	lastPose := spatialmath.NewZeroPose()
	for _, trajNode := range traj {
		if trajNode.Dist > inputs[2].Value {
			lastPose = trajNode.Pose
		} else {
			break
		}
	}

	return lastPose, nil
}

func (pf *ptgFrame) InputFromProtobuf(jp *pb.JointPositions) []referenceframe.Input {
	return nil
}

func (pf *ptgFrame) ProtobufFromInput(inputs []referenceframe.Input) *pb.JointPositions {
	return nil
}

func (pf *ptgFrame) Geometries(inputs []referenceframe.Input) (*referenceframe.GeometriesInFrame, error) {
	return referenceframe.NewGeometriesInFrame(pf.name, pf.geometries), nil
}

func (pf *ptgFrame) AlmostEquals(otherFrame referenceframe.Frame) bool {
	return false
}

func (pf *ptgFrame) PTGs() []tpspace.PTG {
	return pf.ptgs
}

// TODO: this should probably be in referenceframe.
func initPTGs(maxMps, maxDps float64) ([]tpspace.PTG, error) {
	ptgs := []tpspace.PTG{}
	for _, ptg := range defaultPTGs {
		// Forwards version of grid sim
		ptgGen := ptg(maxMps, maxDps, 1.)
		newptg, err := tpspace.NewPTGGridSim(ptgGen, defaultAlphaCnt, defaultSimDist)
		if err != nil {
			return nil, err
		}
		ptgs = append(ptgs, newptg)
		ptgGen = ptg(maxMps, maxDps, -1.)
		if ptgGen != nil {
			// irreversible trajectories, e.g. alpha, will return nil, nil
			newptg, err = tpspace.NewPTGGridSim(ptgGen, defaultAlphaCnt, defaultSimDist)
			if err != nil {
				return nil, err
			}
			ptgs = append(ptgs, newptg)
		}
	}
	return ptgs, nil
}
