// Package motionplan is a motion planning library.
package motionplan

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"sync"
	"time"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// When we generate solutions, if a new solution is within this level of similarity to an existing one, discard it as a duplicate.
// This prevents seeding the solution tree with 50 copies of essentially the same configuration.
const defaultSimScore = 0.05

// motionPlanner provides an interface to path planning methods, providing ways to request a path to be planned, and
// management of the constraints used to plan paths.
type motionPlanner interface {
	// Plan will take a context, a goal position, and an input start state and return a series of state waypoints which
	// should be visited in order to arrive at the goal while satisfying all constraints
	plan(ctx context.Context, seed, goal *PlanState) ([]node, error)

	// Everything below this point should be covered by anything that wraps the generic `planner`
	smoothPath(context.Context, []node) []node
	checkPath(referenceframe.FrameSystemInputs, referenceframe.FrameSystemInputs) bool
	checkInputs(referenceframe.FrameSystemInputs) bool
	getSolutions(context.Context, referenceframe.FrameSystemInputs, ik.StateFSMetric) ([]node, error)
	opt() *PlannerOptions
	sample(node, int) (node, error)
	getScoringFunction() ik.SegmentFSMetric
}

// PlanRequest is a struct to store all the data necessary to make a call to PlanMotion.
type PlanRequest struct {
	// The planner will hit each Goal in order. Each goal may be a configuration or FrameSystemPoses for holonomic motion, or must be a
	// FrameSystemPoses for non-holonomic motion. For holonomic motion, if both a configuration and FrameSystemPoses are given,
	// an error is thrown.
	// TODO: Perhaps we could do something where some components are enforced to arrive at a certain configuration, but others can have IK
	// run to solve for poses. Doing this while enforcing configurations may be tricky.
	Goals []*PlanState `json:"goals"`

	// This must always have a configuration filled in, for geometry placement purposes.
	// If poses are also filled in, the configuration will be used to determine geometry collisions, but the poses will be used
	// in IK to generate plan start configurations. The given configuration will NOT automatically be added to the seed tree.
	// The use case here is that if a particularly difficult path must be planned between two poses, that can be done first to ensure
	// feasibility, and then other plans can be requested to connect to that returned plan's configurations.
	StartState      *PlanState                 `json:"start_state"`
	WorldState      *referenceframe.WorldState `json:"world_state"`
	BoundingRegions []*commonpb.Geometry       `json:"bounding_regions"`
	Constraints     *Constraints               `json:"constraints"`
	PlannerOptions  *PlannerOptions            `json:"planner_options"`
}

// validatePlanRequest ensures PlanRequests are not malformed.
func (req *PlanRequest) validatePlanRequest(fs *referenceframe.FrameSystem) error {
	if req == nil {
		return errors.New("PlanRequest cannot be nil")
	}

	if fs == nil {
		return errors.New("PlanRequest cannot have nil framesystem")
	}
	if req.StartState == nil {
		return errors.New("PlanRequest cannot have nil StartState")
	}
	if req.StartState.configuration == nil {
		return errors.New("PlanRequest cannot have nil StartState configuration")
	}
	// If we have a start configuration, check for correctness. Reuse FrameSystemPoses compute function to provide error.
	if len(req.StartState.configuration) > 0 {
		_, err := req.StartState.configuration.ComputePoses(fs)
		if err != nil {
			return err
		}
	}
	// if we have start poses, check we have valid frames
	for fName, pif := range req.StartState.poses {
		if fs.Frame(fName) == nil {
			return referenceframe.NewFrameMissingError(fName)
		}
		if fs.Frame(pif.Parent()) == nil {
			return referenceframe.NewParentFrameMissingError(fName, pif.Parent())
		}
	}

	if len(req.Goals) == 0 {
		return errors.New("PlanRequest must have at least one goal")
	}

	if req.PlannerOptions.MeshesAsOctrees {
		// convert any meshes in the worldstate to octrees
		if req.WorldState == nil {
			return errors.New("PlanRequest must have non-nil WorldState if 'meshes_as_octrees' option is enabled")
		}
		obstacles := make([]*referenceframe.GeometriesInFrame, 0, len(req.WorldState.ObstacleNames()))
		for _, gf := range req.WorldState.Obstacles() {
			geometries := gf.Geometries()
			pcdGeometries := make([]spatialmath.Geometry, 0, len(geometries))
			for _, geometry := range geometries {
				if mesh, ok := geometry.(*spatialmath.Mesh); ok {
					octree, err := pointcloud.NewFromMesh(mesh)
					if err != nil {
						return err
					}
					geometry = octree
				}
				pcdGeometries = append(pcdGeometries, geometry)
			}
			obstacles = append(obstacles, referenceframe.NewGeometriesInFrame(gf.Parent(), pcdGeometries))
		}
		newWS, err := referenceframe.NewWorldState(obstacles, req.WorldState.Transforms())
		if err != nil {
			return err
		}
		req.WorldState = newWS
	}

	boundingRegions, err := spatialmath.NewGeometriesFromProto(req.BoundingRegions)
	if err != nil {
		return err
	}

	// Validate the goals. Each goal with a pose must not also have a configuration specified. The parent frame of the pose must exist.
	for i, goalState := range req.Goals {
		for fName, pif := range goalState.poses {
			if len(goalState.configuration) > 0 {
				return errors.New("individual goals cannot have both configuration and poses populated")
			}

			goalParentFrame := pif.Parent()
			if fs.Frame(goalParentFrame) == nil {
				return referenceframe.NewParentFrameMissingError(fName, goalParentFrame)
			}

			if len(boundingRegions) > 0 {
				// Check that robot components start within bounding regions.
				// Bounding regions are for 2d planning, which requires a start pose
				if len(goalState.poses) > 0 && len(req.StartState.poses) > 0 {
					goalFrame := fs.Frame(fName)
					if goalFrame == nil {
						return referenceframe.NewFrameMissingError(fName)
					}
					buffer := req.PlannerOptions.CollisionBufferMM
					// check that the request frame's geometries are within or in collision with the bounding regions
					robotGifs, err := goalFrame.Geometries(make([]referenceframe.Input, len(goalFrame.DoF())))
					if err != nil {
						return err
					}
					if i == 0 {
						// Only need to check start poses once
						startPose, ok := req.StartState.poses[fName]
						if !ok {
							return fmt.Errorf("goal frame %s does not have a start pose", fName)
						}
						var robotGeoms []spatialmath.Geometry
						for _, geom := range robotGifs.Geometries() {
							robotGeoms = append(robotGeoms, geom.Transform(startPose.Pose()))
						}
						robotGeomBoundingRegionCheck := NewBoundingRegionConstraint(robotGeoms, boundingRegions, buffer)
						if robotGeomBoundingRegionCheck(&ik.State{}) != nil {
							return fmt.Errorf("frame named %s is not within the provided bounding regions", fName)
						}
					}

					// check that the destination is within or in collision with the bounding regions
					destinationAsGeom := []spatialmath.Geometry{spatialmath.NewPoint(pif.Pose().Point(), "")}
					destinationBoundingRegionCheck := NewBoundingRegionConstraint(destinationAsGeom, boundingRegions, buffer)
					if destinationBoundingRegionCheck(&ik.State{}) != nil {
						return errors.New("destination was not within the provided bounding regions")
					}
				}
			}
		}
	}
	return nil
}

// PlanMotion plans a motion from a provided plan request.
func PlanMotion(ctx context.Context, logger logging.Logger, fs *referenceframe.FrameSystem, request *PlanRequest) (Plan, error) {
	// Calls Replan but without a seed plan
	return Replan(ctx, logger, fs, request, nil, 0)
}

// PlanFrameMotion plans a motion to destination for a given frame with no frame system. It will create a new FS just for the plan.
// WorldState is not supported in the absence of a real frame system.
func PlanFrameMotion(ctx context.Context,
	logger logging.Logger,
	dst spatialmath.Pose,
	f referenceframe.Frame,
	seed []referenceframe.Input,
	constraints *Constraints,
	planningOpts map[string]interface{},
) ([][]referenceframe.Input, error) {
	// ephemerally create a framesystem containing just the frame for the solve
	fs := referenceframe.NewEmptyFrameSystem("")
	if err := fs.AddFrame(f, fs.World()); err != nil {
		return nil, err
	}
	planOpts, err := NewPlannerOptionsFromExtra(planningOpts)
	if err != nil {
		return nil, err
	}
	plan, err := PlanMotion(ctx, logger, fs, &PlanRequest{
		Goals: []*PlanState{
			{poses: referenceframe.FrameSystemPoses{f.Name(): referenceframe.NewPoseInFrame(referenceframe.World, dst)}},
		},
		StartState:     &PlanState{configuration: referenceframe.FrameSystemInputs{f.Name(): seed}},
		Constraints:    constraints,
		PlannerOptions: planOpts,
	})
	if err != nil {
		return nil, err
	}
	return plan.Trajectory().GetFrameInputs(f.Name())
}

// Replan plans a motion from a provided plan request, and then will return that plan only if its cost is better than the cost of the
// passed-in plan multiplied by `replanCostFactor`.
func Replan(
	ctx context.Context,
	logger logging.Logger,
	fs *referenceframe.FrameSystem,
	request *PlanRequest,
	currentPlan Plan,
	replanCostFactor float64,
) (Plan, error) {
	// Make sure request is well formed and not missing vital information
	if err := request.validatePlanRequest(fs); err != nil {
		return nil, err
	}
	logger.CDebugf(ctx, "constraint specs for this step: %v", request.Constraints)
	logger.CDebugf(ctx, "motion config for this step: %v", request.PlannerOptions)

	sfPlanner, err := newPlanManager(logger, fs, request)
	if err != nil {
		return nil, err
	}

	newPlan, err := sfPlanner.planMultiWaypoint(ctx, currentPlan)
	if err != nil {
		return nil, err
	}

	if replanCostFactor > 0 && currentPlan != nil {
		initialPlanCost := currentPlan.Trajectory().EvaluateCost(sfPlanner.scoringFunction)
		finalPlanCost := newPlan.Trajectory().EvaluateCost(sfPlanner.scoringFunction)
		logger.CDebugf(ctx,
			"initialPlanCost %f adjusted with cost factor to %f, replan cost %f",
			initialPlanCost, initialPlanCost*replanCostFactor, finalPlanCost,
		)

		if finalPlanCost > initialPlanCost*replanCostFactor {
			return nil, errHighReplanCost
		}
	}

	return newPlan, nil
}

type planner struct {
	*ConstraintHandler
	fs                        *referenceframe.FrameSystem
	lfs                       *linearizedFrameSystem
	solver                    ik.Solver
	logger                    logging.Logger
	randseed                  *rand.Rand
	start                     time.Time
	scoringFunction           ik.SegmentFSMetric
	poseDistanceFunc          ik.SegmentMetric
	configurationDistanceFunc ik.SegmentFSMetric
	planOpts                  *PlannerOptions
	motionChains              *motionChains
}

func newPlannerFromPlanRequest(logger logging.Logger, fs *referenceframe.FrameSystem, request *PlanRequest) (*planner, error) {
	mChains, err := motionChainsFromPlanState(fs, request.Goals[0])
	if err != nil {
		return nil, err
	}

	// Theoretically, a plan could be made between two poses, by running IK on both the start and end poses to create sets of seed and
	// goal configurations. However, the blocker here is the lack of a "known good" configuration used to determine which obstacles
	// are allowed to collide with one another.
	if !mChains.useTPspace && (request.StartState.configuration == nil) {
		return nil, errors.New("must populate start state configuration if not planning for 2d base/tpspace")
	}

	if mChains.useTPspace {
		if request.StartState.poses == nil {
			return nil, errors.New("must provide a startPose if solving for PTGs")
		}
		if len(request.Goals) != 1 {
			return nil, errors.New("can only provide one goal if solving for PTGs")
		}
	}

	opt, err := updateOptionsForPlanning(request.PlannerOptions, mChains.useTPspace)
	if err != nil {
		return nil, err
	}

	boundingRegions, err := spatialmath.NewGeometriesFromProto(request.BoundingRegions)
	if err != nil {
		return nil, err
	}

	constraintHandler, err := newConstraintHandler(
		opt,
		request.Constraints,
		request.StartState,
		request.Goals[0],
		fs,
		mChains,
		request.StartState.configuration,
		request.WorldState,
		boundingRegions,
	)
	if err != nil {
		return nil, err
	}
	seed := opt.RandomSeed

	//nolint:gosec
	return newPlanner(
		fs,
		rand.New(rand.NewSource(int64(seed))),
		logger,
		opt,
		constraintHandler,
		mChains,
	)
}

func newPlanner(
	fs *referenceframe.FrameSystem,
	seed *rand.Rand,
	logger logging.Logger,
	opt *PlannerOptions,
	constraintHandler *ConstraintHandler,
	chains *motionChains,
) (*planner, error) {
	lfs, err := newLinearizedFrameSystem(fs)
	if err != nil {
		return nil, err
	}
	if opt == nil {
		opt = NewBasicPlannerOptions()
	}
	if constraintHandler == nil {
		constraintHandler = newEmptyConstraintHandler()
	}
	if chains == nil {
		chains = &motionChains{}
	}

	solver, err := ik.CreateCombinedIKSolver(lfs.dof, logger, opt.NumThreads, opt.GoalThreshold)
	if err != nil {
		return nil, err
	}
	mp := &planner{
		ConstraintHandler:         constraintHandler,
		solver:                    solver,
		fs:                        fs,
		lfs:                       lfs,
		logger:                    logger,
		randseed:                  seed,
		planOpts:                  opt,
		scoringFunction:           opt.getScoringFunction(chains),
		poseDistanceFunc:          opt.getPoseDistanceFunc(),
		configurationDistanceFunc: ik.GetConfigurationDistanceFunc(opt.ConfigurationDistanceMetric),
		motionChains:              chains,
	}
	return mp, nil
}

func (mp *planner) checkInputs(inputs referenceframe.FrameSystemInputs) bool {
	return mp.CheckStateFSConstraints(&ik.StateFS{
		Configuration: inputs,
		FS:            mp.fs,
	}) == nil
}

func (mp *planner) checkPath(seedInputs, target referenceframe.FrameSystemInputs) bool {
	ok, _ := mp.CheckSegmentAndStateValidityFS(
		&ik.SegmentFS{
			StartConfiguration: seedInputs,
			EndConfiguration:   target,
			FS:                 mp.fs,
		},
		mp.planOpts.Resolution,
	)
	return ok
}

func (mp *planner) sample(rSeed node, sampleNum int) (node, error) {
	// If we have done more than 50 iterations, start seeding off completely random positions 2 at a time
	// The 2 at a time is to ensure random seeds are added onto both the seed and gofsal maps.
	if sampleNum >= mp.planOpts.IterBeforeRand && sampleNum%4 >= 2 {
		randomInputs := make(referenceframe.FrameSystemInputs)
		for _, name := range mp.fs.FrameNames() {
			f := mp.fs.Frame(name)
			if f != nil && len(f.DoF()) > 0 {
				randomInputs[name] = referenceframe.RandomFrameInputs(f, mp.randseed)
			}
		}
		return newConfigurationNode(randomInputs), nil
	}

	// Seeding nearby to valid points results in much faster convergence in less constrained space
	newInputs := make(referenceframe.FrameSystemInputs)
	for name, inputs := range rSeed.Q() {
		f := mp.fs.Frame(name)
		if f != nil && len(f.DoF()) > 0 {
			q, err := referenceframe.RestrictedRandomFrameInputs(f, mp.randseed, 0.1, inputs)
			if err != nil {
				return nil, err
			}
			newInputs[name] = q
		}
	}
	return newConfigurationNode(newInputs), nil
}

func (mp *planner) opt() *PlannerOptions {
	return mp.planOpts
}

func (mp *planner) getScoringFunction() ik.SegmentFSMetric {
	return mp.scoringFunction
}

// smoothPath will try to naively smooth the path by picking points partway between waypoints and seeing if it can interpolate
// directly between them. This will significantly improve paths from RRT*, as it will shortcut the randomly-selected configurations.
// This will only ever improve paths (or leave them untouched), and runs very quickly.
func (mp *planner) smoothPath(ctx context.Context, path []node) []node {
	mp.logger.CDebugf(ctx, "running simple smoother on path of len %d", len(path))
	if mp.planOpts == nil {
		mp.logger.CDebug(ctx, "nil opts, cannot shortcut")
		return path
	}
	if len(path) <= 2 {
		mp.logger.CDebug(ctx, "path too short, cannot shortcut")
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

		// Use the frame system to interpolate between configurations
		wayPoint1, err := referenceframe.InterpolateFS(mp.fs, path[firstEdge].Q(), path[firstEdge+1].Q(), waypoints[mp.randseed.Intn(3)])
		if err != nil {
			return path
		}
		wayPoint2, err := referenceframe.InterpolateFS(mp.fs, path[secondEdge].Q(), path[secondEdge+1].Q(), waypoints[mp.randseed.Intn(3)])
		if err != nil {
			return path
		}

		if mp.checkPath(wayPoint1, wayPoint2) {
			newpath := []node{}
			newpath = append(newpath, path[:firstEdge+1]...)
			newpath = append(newpath, newConfigurationNode(wayPoint1), newConfigurationNode(wayPoint2))
			// have to split this up due to go compiler quirk where elipses operator can't be mixed with other vars in append
			newpath = append(newpath, path[secondEdge+1:]...)
			path = newpath
		}
	}
	return path
}

// getSolutions will initiate an IK solver for the given position and seed, collect solutions, and score them by constraints.
// If maxSolutions is positive, once that many solutions have been collected, the solver will terminate and return that many solutions.
// If minScore is positive, if a solution scoring below that amount is found, the solver will terminate and return that one solution.
func (mp *planner) getSolutions(ctx context.Context, seed referenceframe.FrameSystemInputs, metric ik.StateFSMetric) ([]node, error) {
	// Linter doesn't properly handle loop labels
	nSolutions := mp.planOpts.MaxSolutions
	if nSolutions == 0 {
		nSolutions = defaultSolutionsToSeed
	}
	if len(seed) == 0 {
		seed = referenceframe.FrameSystemInputs{}
		// If no seed is passed, generate one randomly
		for _, frameName := range mp.fs.FrameNames() {
			seed[frameName] = referenceframe.RandomFrameInputs(mp.fs.Frame(frameName), mp.randseed)
		}
	}

	if metric == nil {
		return nil, errors.New("metric is nil")
	}

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	solutionGen := make(chan *ik.Solution, mp.planOpts.NumThreads*20)
	ikErr := make(chan error, 1)
	var activeSolvers sync.WaitGroup
	defer activeSolvers.Wait()
	activeSolvers.Add(1)
	defer func() {
		// In the case that we have an error, we need to explicitly drain the channel before we return
		for len(solutionGen) > 0 {
			<-solutionGen
		}
	}()

	linearSeed, err := mp.lfs.mapToSlice(seed)
	if err != nil {
		return nil, err
	}

	minFunc := mp.linearizeFSmetric(metric)
	// Spawn the IK solver to generate solutions until done
	utils.PanicCapturingGo(func() {
		defer close(ikErr)
		defer activeSolvers.Done()
		ikErr <- mp.solver.Solve(ctxWithCancel, solutionGen, linearSeed, minFunc, mp.randseed.Int())
	})

	solutions := map[float64]referenceframe.FrameSystemInputs{}

	// A map keeping track of which constraints fail
	failures := map[string]int{}
	constraintFailCnt := 0

	startTime := time.Now()
	firstSolutionTime := time.Hour

	// Solve the IK solver. Loop labels are required because `break` etc in a `select` will break only the `select`.
IK:
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		select {
		case stepSolution := <-solutionGen:

			step, err := mp.lfs.sliceToMap(stepSolution.Configuration)
			if err != nil {
				return nil, err
			}
			alteredStep := mp.nonchainMinimize(seed, step)
			if alteredStep != nil {
				// if nil, step is guaranteed to fail the below check, but we want to do it anyway to capture the failure reason
				step = alteredStep
			}
			// Ensure the end state is a valid one
			err = mp.CheckStateFSConstraints(&ik.StateFS{
				Configuration: step,
				FS:            mp.fs,
			})
			if err == nil {
				stepArc := &ik.SegmentFS{
					StartConfiguration: seed,
					EndConfiguration:   step,
					FS:                 mp.fs,
				}
				err := mp.CheckSegmentFSConstraints(stepArc)
				if err == nil {
					score := mp.configurationDistanceFunc(stepArc)
					if score < mp.planOpts.MinScore && mp.planOpts.MinScore > 0 {
						solutions = map[float64]referenceframe.FrameSystemInputs{}
						solutions[score] = step
						// good solution, stopping early
						break IK
					}
					for _, oldSol := range solutions {
						similarity := &ik.SegmentFS{
							StartConfiguration: oldSol,
							EndConfiguration:   step,
							FS:                 mp.fs,
						}
						simscore := mp.configurationDistanceFunc(similarity)
						if simscore < defaultSimScore {
							continue IK
						}
					}

					solutions[score] = step
					if len(solutions) >= nSolutions {
						// sufficient solutions found, stopping early
						break IK
					}

					if len(solutions) == 1 {
						firstSolutionTime = time.Since(startTime)
					} else {
						elapsed := time.Since(startTime)
						if elapsed > (time.Duration(mp.planOpts.TimeMultipleAfterFindingFirstSolution) * firstSolutionTime) {
							mp.logger.Infof("ending early because of time elapsed: %v firstSolutionTime: %v", elapsed, firstSolutionTime)
							break IK
						}
					}
				} else {
					constraintFailCnt++
					failures[err.Error()]++
				}
			} else {
				constraintFailCnt++
				failures[err.Error()]++
			}
			// Skip the return check below until we have nothing left to read from solutionGen
			continue IK
		default:
		}

		select {
		case <-ikErr:
			// If we have a return from the IK solver, there are no more solutions, so we finish processing above
			// until we've drained the channel, handled by the `continue` above
			break IK
		default:
		}
	}

	// Cancel any ongoing processing within the IK solvers if we're done receiving solutions
	cancel()
	for done := false; !done; {
		select {
		case <-solutionGen:
		default:
			done = true
		}
	}

	if len(solutions) == 0 {
		// We have failed to produce a usable IK solution. Let the user know if zero IK solutions were produced, or if non-zero solutions
		// were produced, which constraints were failed
		if constraintFailCnt == 0 {
			return nil, errIKSolve
		}

		return nil, newIKConstraintErr(failures, constraintFailCnt)
	}

	keys := make([]float64, 0, len(solutions))
	for k := range solutions {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	orderedSolutions := make([]node, 0)
	for _, key := range keys {
		orderedSolutions = append(orderedSolutions, &basicNode{q: solutions[key], cost: key})
	}
	return orderedSolutions, nil
}

// linearize the goal metric for use with solvers.
// Since our solvers operate on arrays of floats, there needs to be a way to map bidirectionally between the framesystem configuration
// of FrameSystemInputs and the []float64 that the solver expects. This is that mapping.
func (mp *planner) linearizeFSmetric(metric ik.StateFSMetric) func([]float64) float64 {
	return func(query []float64) float64 {
		inputs, err := mp.lfs.sliceToMap(query)
		if err != nil {
			return math.Inf(1)
		}
		return metric(&ik.StateFS{Configuration: inputs, FS: mp.fs})
	}
}

// The purpose of this function is to allow solves that require the movement of components not in a motion chain, while preventing wild or
// random motion of these components unnecessarily. A classic example would be a scene with two arms. One arm is given a goal in World
// which it could reach, but the other arm is in the way. Randomly seeded IK will produce a valid configuration for the moving arm, and a
// random configuration for the other. This function attempts to replace that random configuration with the seed configuration, if valid,
// and if invalid will interpolate the solved random configuration towards the seed and set its configuration to the closest valid
// configuration to the seed.
func (mp *planner) nonchainMinimize(seed, step referenceframe.FrameSystemInputs) referenceframe.FrameSystemInputs {
	moving, nonmoving := mp.motionChains.framesFilteredByMovingAndNonmoving(mp.fs)
	// Create a map with nonmoving configurations replaced with their seed values
	alteredStep := referenceframe.FrameSystemInputs{}
	for _, frame := range moving {
		alteredStep[frame] = step[frame]
	}
	for _, frame := range nonmoving {
		alteredStep[frame] = seed[frame]
	}
	if mp.checkInputs(alteredStep) {
		return alteredStep
	}
	// Failing constraints with nonmoving frames at seed. Find the closest passing configuration to seed.

	_, lastGood := mp.CheckStateConstraintsAcrossSegmentFS(
		&ik.SegmentFS{
			StartConfiguration: step,
			EndConfiguration:   alteredStep,
			FS:                 mp.fs,
		}, mp.planOpts.Resolution,
	)
	if lastGood != nil {
		return lastGood.EndConfiguration
	}
	return nil
}
