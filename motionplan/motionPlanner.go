// Package motionplan is a motion planning library.
package motionplan

import (
	"context"
	"math/rand"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/spatialmath"
)

// motionPlanner provides an interface to path planning methods, providing ways to request a path to be planned, and
// management of the constraints used to plan paths.
type motionPlanner interface {
	// Plan will take a context, a goal position, and an input start state and return a series of state waypoints which
	// should be visited in order to arrive at the goal while satisfying all constraints
	plan(context.Context, spatialmath.Pose, []frame.Input) ([][]frame.Input, error)

	// Everything below this point should be covered by anything that wraps the generic `planner`
	smoothPath(context.Context, []node) []node
	checkPath([]frame.Input, []frame.Input) bool
	checkInputs([]frame.Input) bool
}

type parallelMotionPlanner interface {
	motionPlanner
	planParallel(context.Context, spatialmath.Pose, []frame.Input, chan<- *rrtPlanReturn) // TODO(rb): make planReturn an interface
}

type plannerConstructor func(frame.Frame, *rand.Rand, golog.Logger, *plannerOptions) (motionPlanner, error)

// PlanMotion plans a motion to destination for a given frame. It takes a given frame system, wraps it with a SolvableFS, and solves.
func PlanMotion(ctx context.Context,
	logger golog.Logger,
	dst *frame.PoseInFrame,
	f frame.Frame,
	seedMap map[string][]frame.Input,
	fs frame.FrameSystem,
	worldState *frame.WorldState,
	planningOpts map[string]interface{},
) ([]map[string][]frame.Input, error) {
	return PlanWaypoints(ctx, logger, []*frame.PoseInFrame{dst}, f, seedMap, fs, worldState, []map[string]interface{}{planningOpts})
}

// PlanRobotMotion plans a motion to destination for a given frame. A robot object is passed in and current position inputs are determined.
func PlanRobotMotion(ctx context.Context,
	dst *frame.PoseInFrame,
	f frame.Frame,
	r robot.Robot,
	fs frame.FrameSystem,
	worldState *frame.WorldState,
	planningOpts map[string]interface{},
) ([]map[string][]frame.Input, error) {
	seedMap, _, err := framesystem.RobotFsCurrentInputs(ctx, r, fs)
	if err != nil {
		return nil, err
	}

	return PlanWaypoints(ctx, r.Logger(), []*frame.PoseInFrame{dst}, f, seedMap, fs, worldState, []map[string]interface{}{planningOpts})
}

// PlanFrameMotion plans a motion to destination for a given frame with no frame system. It will create a new FS just for the plan.
// WorldState is not supported in the absence of a real frame system.
func PlanFrameMotion(ctx context.Context,
	logger golog.Logger,
	dst spatialmath.Pose,
	f frame.Frame,
	seed []frame.Input,
	planningOpts map[string]interface{},
) ([][]frame.Input, error) {
	// ephemerally create a framesystem containing just the frame for the solve
	fs := frame.NewEmptySimpleFrameSystem("")
	err := fs.AddFrame(f, fs.World())
	if err != nil {
		return nil, err
	}
	destination := frame.NewPoseInFrame(frame.World, dst)
	seedMap := map[string][]frame.Input{f.Name(): seed}
	solutionMap, err := PlanWaypoints(
		ctx,
		logger,
		[]*frame.PoseInFrame{destination},
		f,
		seedMap,
		fs,
		nil,
		[]map[string]interface{}{planningOpts},
	)
	if err != nil {
		return nil, err
	}
	return FrameStepsFromRobotPath(f.Name(), solutionMap)
}

// PlanWaypoints plans motions to a list of destinations in order for a given frame. It takes a given frame system, wraps it with a
// SolvableFS, and solves. It will generate a list of intermediate waypoints as well to pass to the solvable framesystem if possible.
func PlanWaypoints(ctx context.Context,
	logger golog.Logger,
	goals []*frame.PoseInFrame,
	f frame.Frame,
	seedMap map[string][]frame.Input,
	fs frame.FrameSystem,
	worldState *frame.WorldState,
	motionConfigs []map[string]interface{},
) ([]map[string][]frame.Input, error) {
	if len(goals) == 0 {
		return nil, errors.New("no destinations passed to PlanWaypoints")
	}

	steps := make([]map[string][]frame.Input, 0, len(goals)*2)

	// Get parentage of solver frame. This will also verify the frame is in the frame system
	solveFrame := fs.Frame(f.Name())
	if solveFrame == nil {
		return nil, frame.NewFrameMissingError(f.Name())
	}
	solveFrameList, err := fs.TracebackFrame(solveFrame)
	if err != nil {
		return nil, err
	}

	// If no planning opts, use default. If one, use for all goals. If one per goal, use respective option. Otherwise error.
	configs := make([]map[string]interface{}, 0, len(goals))
	if len(motionConfigs) != len(goals) {
		switch len(motionConfigs) {
		case 0:
			for range goals {
				configs = append(configs, map[string]interface{}{})
			}
		case 1:
			// If one config passed, use it for all waypoints
			for range goals {
				configs = append(configs, motionConfigs[0])
			}
		default:
			return nil, errors.New("goals and motion configs had different lengths")
		}
	} else {
		configs = motionConfigs
	}

	// Each goal is a different PoseInFrame and so may have a different destination Frame. Since the motion can be solved from either end,
	// each goal is solved independently.
	for i, goal := range goals {
		// Create a frame to solve for, and an IK solver with that frame.
		sf, err := newSolverFrame(fs, solveFrameList, goal.Parent(), seedMap)
		if err != nil {
			return nil, err
		}
		if len(sf.DoF()) == 0 {
			return nil, errors.New("solver frame has no degrees of freedom, cannot perform inverse kinematics")
		}

		manager, err := newPlanManager(logger, fs, sf, seedMap, goal.Pose(), worldState, i, configs[i])
		if err != nil {
			return nil, err
		}
		resultSlices, err := manager.PlanSingleWaypoint(ctx)
		if err != nil {
			return nil, err
		}
		for j, resultSlice := range resultSlices {
			stepMap := sf.inputToMap(resultSlice)
			steps = append(steps, stepMap)
			if j == len(resultSlices)-1 {
				// update seed map
				seedMap = stepMap
			}
		}
	}

	return steps, nil
}

type planner struct {
	ik       inverseKinematicsSolver
	frame    frame.Frame
	logger   golog.Logger
	randseed *rand.Rand
	start    time.Time
	planOpts *plannerOptions
}

func newPlanner(frame frame.Frame, seed *rand.Rand, logger golog.Logger, opt *plannerOptions) (*planner, error) {
	ik, err := newIKSolver(frame, logger, opt.ikOptions)
	if err != nil {
		return nil, err
	}
	mp := &planner{
		ik:       ik,
		frame:    frame,
		logger:   logger,
		randseed: seed,
		planOpts: opt,
	}
	return mp, nil
}

func (mp *planner) checkInputs(inputs []frame.Input) bool {
	position, err := mp.frame.Transform(inputs)
	if err != nil {
		return false
	}
	ok, _ := mp.planOpts.CheckConstraints(&ConstraintInput{
		StartPos:   position,
		EndPos:     position,
		StartInput: inputs,
		EndInput:   inputs,
		Frame:      mp.frame,
	})
	return ok
}

func (mp *planner) checkPath(seedInputs, target []frame.Input) bool {
	ok, _ := mp.planOpts.CheckConstraintPath(
		&ConstraintInput{
			StartInput: seedInputs,
			EndInput:   target,
			Frame:      mp.frame,
		},
		mp.planOpts.Resolution,
	)
	return ok
}

// smoothPath will try to naively smooth the path by picking points partway between waypoints and seeing if it can interpolate
// directly between them. This will significantly improve paths from RRT*, as it will shortcut the randomly-selected configurations.
// This will only ever improve paths (or leave them untouched), and runs very quickly.
func (mp *planner) smoothPath(ctx context.Context, path []node) []node {
	mp.logger.Debugf("running simple smoother on path of len %d", len(path))
	if mp.planOpts == nil {
		mp.logger.Debug("nil opts, cannot shortcut")
		return path
	}
	if len(path) <= 2 {
		mp.logger.Debug("path too short, cannot shortcut")
		return path
	}

	// Randomly pick which quarter of motion to check from; this increases flexibility of smoothing.
	waypoints := []float64{0.25, 0.5, 0.75}

	for i := 0; i < mp.planOpts.SmoothIter; i++ {
		select {
		case <-ctx.Done():
			return path
		default:
		}
		// get start node of first edge. Cannot be either the last or second-to-last node.
		// Intn will return an int in the half-open interval half-open interval [0,n)
		firstEdge := mp.randseed.Intn(len(path) - 2)
		secondEdge := firstEdge + 1 + mp.randseed.Intn((len(path)-2)-firstEdge)
		mp.logger.Debugf("checking shortcut between nodes %d and %d", firstEdge, secondEdge+1)

		wayPoint1 := frame.InterpolateInputs(path[firstEdge].Q(), path[firstEdge+1].Q(), waypoints[mp.randseed.Intn(3)])
		wayPoint2 := frame.InterpolateInputs(path[secondEdge].Q(), path[secondEdge+1].Q(), waypoints[mp.randseed.Intn(3)])

		if mp.checkPath(wayPoint1, wayPoint2) {
			newpath := []node{}
			newpath = append(newpath, path[:firstEdge+1]...)
			newpath = append(newpath, &basicNode{wayPoint1}, &basicNode{wayPoint2})
			// have to split this up due to go compiler quirk where elipses operator can't be mixed with other vars in append
			newpath = append(newpath, path[secondEdge+1:]...)
			path = newpath
		}
	}
	return path
}
