package armplanning

import (
	"context"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// gantryCell is a UR20 on a 3 m gantry along X. The arm swings its base joint from reaching +Y at
// gantry 0 to reaching -Y at gantry 3000.
type gantryCell struct {
	fs          *referenceframe.FrameSystem
	start, goal referenceframe.FrameSystemInputs
	// gantryFirst and armFirst are the configurations reached after moving only that component.
	gantryFirst, armFirst referenceframe.FrameSystemInputs
}

func newGantryCell(t *testing.T) *gantryCell {
	t.Helper()
	gantry, err := referenceframe.NewTranslationalFrame("gantry", r3.Vector{X: 1}, referenceframe.Limit{Min: 0, Max: 3000})
	test.That(t, err, test.ShouldBeNil)
	armModel, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur20.json"), "arm")
	test.That(t, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("gantry-cell")
	test.That(t, fs.AddFrame(gantry, fs.World()), test.ShouldBeNil)
	test.That(t, fs.AddFrame(armModel, gantry), test.ShouldBeNil)

	armStart := []referenceframe.Input{math.Pi / 2, -0.4, 0.8, 0, 0, 0}
	armGoal := []referenceframe.Input{-math.Pi / 2, -0.4, 0.8, 0, 0, 0}
	return &gantryCell{
		fs:          fs,
		start:       referenceframe.FrameSystemInputs{"gantry": {0}, "arm": armStart},
		goal:        referenceframe.FrameSystemInputs{"gantry": {3000}, "arm": armGoal},
		gantryFirst: referenceframe.FrameSystemInputs{"gantry": {3000}, "arm": armStart},
		armFirst:    referenceframe.FrameSystemInputs{"gantry": {0}, "arm": armGoal},
	}
}

// boxAtForearm is a 200 mm box where the arm's outermost link is in cfg.
func (c *gantryCell) boxAtForearm(t *testing.T, cfg referenceframe.FrameSystemInputs, label string) spatialmath.Geometry {
	t.Helper()
	geoms, err := referenceframe.FrameSystemGeometries(c.fs, cfg)
	test.That(t, err, test.ShouldBeNil)
	base := r3.Vector{X: cfg["gantry"][0]}
	tip := base
	for _, g := range geoms["arm"].Geometries() {
		if p := g.Pose().Point(); p.Sub(base).Norm() > tip.Sub(base).Norm() {
			tip = p
		}
	}
	box, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(tip), r3.Vector{X: 200, Y: 200, Z: 200}, label)
	test.That(t, err, test.ShouldBeNil)
	return box
}

func (c *gantryCell) plan(t *testing.T, obstacles []spatialmath.Geometry) ([]referenceframe.FrameSystemInputs, error) {
	t.Helper()
	plan, _, err := PlanMotion(context.Background(), logging.NewTestLogger(t), &PlanRequest{
		FrameSystem:           c.fs,
		StartState:            NewPlanState(nil, c.start),
		Goals:                 []*PlanState{NewPlanState(nil, c.goal)},
		ObstaclesInWorldFrame: referenceframe.NewGeometriesInFrame(referenceframe.World, obstacles),
		PlannerOptions:        NewBasicPlannerOptions(),
	})
	if err != nil {
		return nil, err
	}
	return plan.Trajectory(), nil
}

// assertSingleComponentAndClear checks that every leg moves one frame and that driving the legs in
// order, one frame at a time, never touches an obstacle.
func (c *gantryCell) assertSingleComponentAndClear(
	t *testing.T, traj []referenceframe.FrameSystemInputs, obstacles []spatialmath.Geometry,
) {
	t.Helper()
	for i := 1; i < len(traj); i++ {
		from, to := traj[i-1].ToLinearInputs(), traj[i].ToLinearInputs()
		test.That(t, len(MovedFrames(from, to)), test.ShouldEqual, 1)
		for k := 0; k <= 100; k++ {
			cfg := referenceframe.FrameSystemInputs{}
			for name := range traj[i] {
				interp, err := c.fs.Frame(name).Interpolate(traj[i-1][name], traj[i][name], float64(k)/100)
				test.That(t, err, test.ShouldBeNil)
				cfg[name] = interp
			}
			geoms, err := referenceframe.FrameSystemGeometries(c.fs, cfg)
			test.That(t, err, test.ShouldBeNil)
			for _, g := range geoms["arm"].Geometries() {
				for _, o := range obstacles {
					hit, _, err := g.CollidesWith(o, 0)
					test.That(t, err, test.ShouldBeNil)
					test.That(t, hit, test.ShouldBeFalse)
				}
			}
		}
	}
	test.That(t, traj[len(traj)-1]["gantry"], test.ShouldResemble, c.goal["gantry"])
	test.That(t, traj[len(traj)-1]["arm"], test.ShouldResemble, c.goal["arm"])
}

func TestSequentializeNoObstacles(t *testing.T) {
	c := newGantryCell(t)
	traj, err := c.plan(t, nil)
	test.That(t, err, test.ShouldBeNil)
	// Frames are tried in name order, so the arm moves first.
	test.That(t, len(traj), test.ShouldEqual, 3)
	test.That(t, traj[1]["arm"], test.ShouldResemble, c.goal["arm"])
	test.That(t, traj[1]["gantry"], test.ShouldResemble, c.start["gantry"])
	c.assertSingleComponentAndClear(t, traj, nil)
}

func TestSequentializePicksTheOrderThatIsClear(t *testing.T) {
	c := newGantryCell(t)
	obstacles := []spatialmath.Geometry{c.boxAtForearm(t, c.armFirst, "blocks-arm-first")}
	traj, err := c.plan(t, obstacles)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(traj), test.ShouldEqual, 3)
	test.That(t, traj[1]["gantry"], test.ShouldResemble, c.goal["gantry"])
	test.That(t, traj[1]["arm"], test.ShouldResemble, c.start["arm"])
	c.assertSingleComponentAndClear(t, traj, obstacles)
}

func TestSequentializeSplitsWhenNeitherOrderIsClear(t *testing.T) {
	c := newGantryCell(t)
	obstacles := []spatialmath.Geometry{
		c.boxAtForearm(t, c.armFirst, "blocks-arm-first"),
		c.boxAtForearm(t, c.gantryFirst, "blocks-gantry-first"),
	}
	traj, err := c.plan(t, obstacles)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(traj), test.ShouldBeGreaterThan, 3)
	for i, step := range traj {
		t.Logf("waypoint %d: gantry %v arm %v", i, step["gantry"], step["arm"])
	}
	c.assertSingleComponentAndClear(t, traj, obstacles)
}

func TestSequentializeLeavesSingleComponentPlansAlone(t *testing.T) {
	c := newGantryCell(t)
	c.goal = c.gantryFirst
	traj, err := c.plan(t, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(traj), test.ShouldEqual, 2)
}

func TestFrameOrders(t *testing.T) {
	test.That(t, frameOrders([]string{"a", "b"}), test.ShouldResemble, [][]string{{"a", "b"}, {"b", "a"}})
	test.That(t, len(frameOrders([]string{"a", "b", "c"})), test.ShouldEqual, 6)
	test.That(t, frameOrders([]string{"a", "b", "c", "d"}), test.ShouldResemble,
		[][]string{{"a", "b", "c", "d"}, {"d", "c", "b", "a"}})
}
