package builtin

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// recordingIE is an InputEnabled component that appends every GoToInputs call to a shared log.
type recordingIE struct {
	resource.Resource
	name string
	curr []referenceframe.Input
	log  *[]string
}

func (r *recordingIE) Kinematics(context.Context) (referenceframe.Model, error) { return nil, nil }

func (r *recordingIE) CurrentInputs(context.Context) ([]referenceframe.Input, error) {
	return r.curr, nil
}

func (r *recordingIE) GoToInputs(_ context.Context, steps ...[]referenceframe.Input) error {
	*r.log = append(*r.log, fmt.Sprintf("%s%v", r.name, steps))
	r.curr = steps[len(steps)-1]
	return nil
}

func executeRecorded(traj motionplan.Trajectory) (string, error) {
	log := []string{}
	ms := &builtIn{components: map[string]resource.Resource{
		"arm":    &recordingIE{name: "arm", curr: traj[0]["arm"], log: &log},
		"gantry": &recordingIE{name: "gantry", curr: traj[0]["gantry"], log: &log},
	}}
	err := ms.execute(context.Background(), traj, math.MaxFloat64)
	return strings.Join(log, " -> "), err
}

func TestExecuteRunsComponentMovesInPlanOrder(t *testing.T) {
	traj := motionplan.Trajectory{
		{"arm": {0, 0}, "gantry": {200}},
		{"arm": {1, 1}, "gantry": {200}},
		{"arm": {1, 1}, "gantry": {2994.5}},
		{"arm": {2, 2}, "gantry": {2994.5}},
	}
	// Map iteration order is random, so one run proves nothing about ordering.
	for range 200 {
		calls, err := executeRecorded(traj)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, calls, test.ShouldEqual, "arm[[1 1]] -> gantry[[2994.5]] -> arm[[2 2]]")
	}
}

func TestExecuteBatchesConsecutiveWaypointsOfOneComponent(t *testing.T) {
	calls, err := executeRecorded(motionplan.Trajectory{
		{"arm": {0, 0}, "gantry": {200}},
		{"arm": {1, 1}, "gantry": {200}},
		{"arm": {2, 2}, "gantry": {200}},
		{"arm": {2, 2}, "gantry": {300}},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, calls, test.ShouldEqual, "arm[[1 1] [2 2]] -> gantry[[300]]")
}

func TestExecuteRefusesWaypointThatMovesTwoComponents(t *testing.T) {
	calls, err := executeRecorded(motionplan.Trajectory{
		{"arm": {0, 0}, "gantry": {200}},
		{"arm": {1, 1}, "gantry": {2994.5}},
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "moves both arm and gantry")
	test.That(t, calls, test.ShouldEqual, "")
}

func TestExecuteIgnoresChangesBelowEpsilon(t *testing.T) {
	calls, err := executeRecorded(motionplan.Trajectory{
		{"arm": {0.0626375, 0}, "gantry": {200}},
		{"arm": {0.06265, 0}, "gantry": {759}},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, calls, test.ShouldEqual, "gantry[[759]]")
}

// simCell holds the live inputs of simulated components and checks the arm against the obstacles
// each time one of them advances.
type simCell struct {
	fs        *referenceframe.FrameSystem
	obstacles []spatialmath.Geometry
	state     referenceframe.FrameSystemInputs
}

var errSimCollision = errors.New("simulated collision")

// simIE moves linearly in joint space to each commanded target and blocks until it arrives.
type simIE struct {
	resource.Resource
	name string
	cell *simCell
}

func (s *simIE) Kinematics(context.Context) (referenceframe.Model, error) { return nil, nil }

func (s *simIE) CurrentInputs(context.Context) ([]referenceframe.Input, error) {
	return s.cell.state[s.name], nil
}

func (s *simIE) GoToInputs(_ context.Context, steps ...[]referenceframe.Input) error {
	for _, target := range steps {
		from := s.cell.state[s.name]
		for i := 1; i <= 100; i++ {
			curr, err := s.cell.fs.Frame(s.name).Interpolate(from, target, float64(i)/100)
			if err != nil {
				return err
			}
			s.cell.state[s.name] = curr
			geoms, err := referenceframe.FrameSystemGeometries(s.cell.fs, s.cell.state)
			if err != nil {
				return err
			}
			for _, g := range geoms["arm"].Geometries() {
				for _, o := range s.cell.obstacles {
					hit, _, err := g.CollidesWith(o, 0)
					if err != nil {
						return err
					}
					if hit {
						return fmt.Errorf("%w: %s hits %s while %s moves", errSimCollision, g.Label(), o.Label(), s.name)
					}
				}
			}
		}
	}
	return nil
}

// A UR20 on a gantry swings its base joint half a turn while the carriage travels 3 m. A box sits
// at each configuration reached by moving only the arm or only the carriage, so the plan is only
// safe to execute if the planner accounted for the components moving one at a time.
func TestPlanAndExecuteArmOnGantryOneComponentAtATime(t *testing.T) {
	gantry, err := referenceframe.NewTranslationalFrame("gantry", r3.Vector{X: 1}, referenceframe.Limit{Min: 0, Max: 3000})
	test.That(t, err, test.ShouldBeNil)
	armModel, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur20.json"), "arm")
	test.That(t, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("gantry-cell")
	test.That(t, fs.AddFrame(gantry, fs.World()), test.ShouldBeNil)
	test.That(t, fs.AddFrame(armModel, gantry), test.ShouldBeNil)

	armStart := []referenceframe.Input{math.Pi / 2, -0.4, 0.8, 0, 0, 0}
	armGoal := []referenceframe.Input{-math.Pi / 2, -0.4, 0.8, 0, 0, 0}
	start := referenceframe.FrameSystemInputs{"gantry": {0}, "arm": armStart}
	goal := referenceframe.FrameSystemInputs{"gantry": {3000}, "arm": armGoal}

	boxAtForearm := func(cfg referenceframe.FrameSystemInputs, label string) spatialmath.Geometry {
		geoms, err := referenceframe.FrameSystemGeometries(fs, cfg)
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
	obstacles := []spatialmath.Geometry{
		boxAtForearm(referenceframe.FrameSystemInputs{"gantry": {0}, "arm": armGoal}, "blocks-arm-first"),
		boxAtForearm(referenceframe.FrameSystemInputs{"gantry": {3000}, "arm": armStart}, "blocks-gantry-first"),
	}

	plan, _, err := armplanning.PlanMotion(context.Background(), logging.NewTestLogger(t), &armplanning.PlanRequest{
		FrameSystem:           fs,
		StartState:            armplanning.NewPlanState(nil, start),
		Goals:                 []*armplanning.PlanState{armplanning.NewPlanState(nil, goal)},
		ObstaclesInWorldFrame: referenceframe.NewGeometriesInFrame(referenceframe.World, obstacles),
		PlannerOptions:        armplanning.NewBasicPlannerOptions(),
	})
	test.That(t, err, test.ShouldBeNil)

	for range 50 {
		cell := &simCell{fs: fs, obstacles: obstacles, state: referenceframe.FrameSystemInputs{"gantry": {0}, "arm": armStart}}
		ms := &builtIn{components: map[string]resource.Resource{
			"arm":    &simIE{name: "arm", cell: cell},
			"gantry": &simIE{name: "gantry", cell: cell},
		}}
		test.That(t, ms.execute(context.Background(), plan.Trajectory(), defaultExecuteEpsilon), test.ShouldBeNil)
		test.That(t, cell.state["gantry"], test.ShouldResemble, goal["gantry"])
		test.That(t, cell.state["arm"], test.ShouldResemble, goal["arm"])
	}
}
