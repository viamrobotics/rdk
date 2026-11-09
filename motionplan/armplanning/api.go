// Package armplanning is a motion planning library.
package armplanning

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"
	"go.viam.com/utils/trace"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

type testOptions struct {
	doNotCloseObstacles bool
}

// PlanRequest is a struct to store all the data necessary to make a call to PlanMotion.
type PlanRequest struct {
	FrameSystem *referenceframe.FrameSystem `json:"frame_system"`

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
	StartState *PlanState `json:"start_state"`
	// The data representation of the robot's environment.
	ObstaclesInWorldFrame *referenceframe.GeometriesInFrame `json:"obstacles_in_world_frame"`
	// Additional parameters constraining the motion of the robot.
	Constraints *motionplan.Constraints `json:"constraints"`
	// Other more granular parameters for the plan used to move the robot.
	PlannerOptions *PlannerOptions `json:"planner_options"`

	rejectedSolutions map[*PlanState][]referenceframe.FrameSystemInputs
	myTestOptions     testOptions
}

// GetWorldState is a backwards compatibility helper that turns the ObstaclesInWorldFrame back into
// WorldState object. Presumably for interfacing/rendering with the viz-client.
func (req *PlanRequest) GetWorldState() *referenceframe.WorldState {
	var obstacles []*referenceframe.GeometriesInFrame
	if req.ObstaclesInWorldFrame != nil {
		obstacles = append(obstacles, req.ObstaclesInWorldFrame)
	}
	return mustNewWorldState(obstacles, nil)
}

// validatePlanRequest ensures PlanRequests are not malformed.
func (req *PlanRequest) validatePlanRequest() error {
	if req == nil {
		return errors.New("PlanRequest cannot be nil")
	}
	if req.FrameSystem == nil {
		return errors.New("PlanRequest cannot have nil framesystem")
	}

	if req.StartState == nil {
		return errors.New("PlanRequest cannot have nil StartState")
	}
	if req.StartState.structuredConfiguration == nil {
		return errors.New("PlanRequest cannot have nil StartState configuration")
	}
	if req.PlannerOptions == nil {
		req.PlannerOptions = NewBasicPlannerOptions()
	}
	if req.PlannerOptions.CollisionBufferMM <= 0 {
		return errors.New("collision_buffer_mm has to be positive")
	}

	// If we have a start configuration, check for correctness. Reuse FrameSystemPoses compute function to provide error.
	if len(req.StartState.structuredConfiguration) > 0 {
		_, err := req.StartState.structuredConfiguration.ComputePoses(req.FrameSystem)
		if err != nil {
			return err
		}
	}

	// if we have start poses, check we have valid frames
	for fName, pif := range req.StartState.poses {
		if req.FrameSystem.Frame(fName) == nil {
			return referenceframe.NewFrameMissingError(fName)
		}
		if req.FrameSystem.Frame(pif.Parent()) == nil {
			return referenceframe.NewParentFrameMissingError(fName, pif.Parent())
		}
	}

	if req.ObstaclesInWorldFrame != nil && req.ObstaclesInWorldFrame.Parent() != referenceframe.World {
		return errors.Errorf("ObstaclesInWorldFrame must be parented by %q, got %q",
			referenceframe.World, req.ObstaclesInWorldFrame.Parent())
	}

	if len(req.Goals) == 0 {
		return errors.New("PlanRequest must have at least one goal")
	}

	if req.PlannerOptions.MeshesAsOctrees {
		// convert any meshes in the obstacles to octrees
		if req.ObstaclesInWorldFrame == nil {
			return errors.New("PlanRequest must have non-nil ObstaclesInWorldFrame if 'meshes_as_octrees' option is enabled")
		}

		pcdGeometries := make([]spatialmath.Geometry, 0, len(req.ObstaclesInWorldFrame.Geometries()))
		for _, geometry := range req.ObstaclesInWorldFrame.Geometries() {
			if mesh, ok := geometry.(*spatialmath.Mesh); ok {
				octree, err := pointcloud.NewFromMesh(mesh)
				if err != nil {
					return err
				}

				geometry = octree
			}

			pcdGeometries = append(pcdGeometries, geometry)
		}

		req.ObstaclesInWorldFrame = referenceframe.NewGeometriesInFrame(
			req.ObstaclesInWorldFrame.Parent(), pcdGeometries,
		)
	}

	// Validate the goals. Each goal with a pose must not also have a configuration specified. The parent frame of the pose must exist.
	for _, goalState := range req.Goals {
		for fName, pif := range goalState.poses {
			if len(goalState.structuredConfiguration) > 0 {
				return errors.New("individual goals cannot have both configuration and poses populated")
			}

			goalParentFrame := pif.Parent()
			if req.FrameSystem.Frame(goalParentFrame) == nil {
				return referenceframe.NewParentFrameMissingError(fName, goalParentFrame)
			}
		}
	}

	if req.Constraints == nil {
		req.Constraints = &motionplan.Constraints{}
	}
	return nil
}

// PlanFrameMotion plans a motion to destination for a given frame with no frame system. It will create a new FS just for the plan.
// WorldState is not supported in the absence of a real frame system.
func PlanFrameMotion(ctx context.Context,
	logger logging.Logger,
	dst spatialmath.Pose,
	f referenceframe.Frame,
	seed []referenceframe.Input,
	constraints *motionplan.Constraints,
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
	plan, _, err := PlanMotion(ctx, logger, &PlanRequest{
		FrameSystem: fs,
		Goals: []*PlanState{
			{poses: referenceframe.FrameSystemPoses{f.Name(): referenceframe.NewPoseInFrame(referenceframe.World, dst)}},
		},
		StartState:     &PlanState{structuredConfiguration: referenceframe.FrameSystemInputs{f.Name(): seed}},
		Constraints:    constraints,
		PlannerOptions: planOpts,
	})
	if err != nil {
		return nil, err
	}
	return plan.Trajectory().GetFrameInputs(f.Name())
}

// SolutionNodeInfo captures per-node data from getSolutions for visualization and debugging.
type SolutionNodeInfo struct {
	// Score is the cost of moving from the start configuration to this node.
	Score float64
	// CheckPathError is nil when the straight-line path from start to this node passed all
	// constraints, or the constraint violation error otherwise.
	CheckPathError error
	// Inputs is the goal configuration for this IK solution.
	Inputs *referenceframe.LinearInputs
	// LastGoodInputs is the last interpolated configuration before the checkPath failure.
	// Nil when CheckPathError is nil.
	LastGoodInputs *referenceframe.LinearInputs
}

// PerGoalMeta holds diagnostic data for a single invocation of initRRTSolutions.
// Only populated when PlannerOptions.CollectSolutionDiagnostics is true.
type PerGoalMeta struct {
	// StartConfiguration is the start configuration.
	StartConfiguration *referenceframe.LinearInputs

	// GoalPoses are the goal poses.
	GoalPoses referenceframe.FrameSystemPoses

	// ReasonableCost is what's used to filter out IK solutions that require much more joint
	// movement than we expect should be necessary.
	ReasonableCost float64

	// SolutionNodes contains info about each IK solution node scored and path-checked.
	SolutionNodes []SolutionNodeInfo

	// ConstraintFailuresByType maps constraint error strings to the number of IK candidate
	// solutions that failed that constraint.
	ConstraintFailuresByType map[string]int
}

// PlanMeta is meta data about plan generation. All numeric values are summed over
// `BacktrackCounter`. One can take divide the numeric values by `BacktrackCounter` to get an
// average over each of the plan executions. When using `PlanMotionSequence`, there's still one
// `PlanMeta` for each `PlanRequest`, but we aggregate numbers in a best-effort manner as we try
// solving plans from different start states.
type PlanMeta struct {
	// Duration is how long the plan took to generate.
	Duration time.Duration

	// Partial is true if we're returning a trajectory that satisfies a prefix of the requested
	// goals. As opposed to satisfying all of the goals.
	Partial bool

	// PartialError includes the error that halted execution when the algorithm decided to return a
	// partial answer.
	PartialError error

	// GoalsProcessed is how many user-defined goals were solved for.
	GoalsProcessed int

	// GoalsCBIRRTSolved returns the number of waypoints that CBIRRT solved for.
	GoalsCBIRRTSolved int

	// GoalsNudgeSolved is the number of waypoints solved by repairing the
	// straight-line path with small nudges around obstacles, skipping CBIRRT.
	GoalsNudgeSolved int

	// GoalsRoadmapSolved is the number of waypoints solved through the lazy
	// roadmap, skipping CBIRRT.
	GoalsRoadmapSolved int

	// SubgoalsPerGoal will have size of `GoalsProcessed`. If there are no linear/orientation
	// constraints, we do not create any additional subgoals/waypoints. SubgoalsPerGoal in that case
	// will be set to 1 for each goal index. Otherwise it will be sent to the number of internal
	// waypoints created + 1 (for the final user goal).
	SubgoalsPerGoal []int

	// SubgoalsProcessed may be non-zero when a plan request has linear/orientation
	// constraints. Satisfying those constraints internally creates additional goals.
	SubgoalsProcessed int

	// CollectSolutionDiagnostics is copied from PlannerOptions and gates whether PerGoal is
	// populated.
	CollectSolutionDiagnostics bool

	// PerGoal holds diagnostic data indexed by initRRTSolutions invocation order. Each top-level
	// goal, sub-goal, and planning split produces one entry.
	PerGoal []PerGoalMeta

	// BacktrackCounter is incremented each time `PlanMotionSequence` re-attempts the corresponding
	// PlanRequest in hopes for a better end result.
	BacktrackCounter int
}

// Merge will sum all of the numeric members of `PlanMeta`. Non-numeric members will be overwritten.
func (pm *PlanMeta) Merge(other PlanMeta) *PlanMeta {
	if pm == nil {
		return &other
	}

	pm.Duration += other.Duration
	pm.GoalsProcessed += other.GoalsProcessed
	pm.GoalsCBIRRTSolved += other.GoalsCBIRRTSolved
	pm.GoalsNudgeSolved += other.GoalsNudgeSolved
	pm.GoalsRoadmapSolved += other.GoalsRoadmapSolved
	pm.SubgoalsProcessed += other.SubgoalsProcessed
	pm.BacktrackCounter += other.BacktrackCounter

	pm.Partial = other.Partial
	pm.PartialError = other.PartialError
	pm.SubgoalsPerGoal = other.SubgoalsPerGoal
	pm.CollectSolutionDiagnostics = other.CollectSolutionDiagnostics
	pm.PerGoal = other.PerGoal

	return pm
}

// PlanMotion plans a motion from a provided plan request. PlanMeta is always a valid pointer.
func PlanMotion(ctx context.Context, parentLogger logging.Logger, request *PlanRequest) (motionplan.Plan, *PlanMeta, error) {
	logger := parentLogger.Sublogger("mp")

	start := time.Now()
	meta := &PlanMeta{}
	ctx, span := trace.StartSpan(ctx, "PlanMotion")
	defer func() {
		meta.Duration = time.Since(start)
		// We don't expect mesh caches to be useful from plan to plan, even when the start and goal
		// states are exactly the same due to differing IK solutions (this was backed by
		// profiling). Hence we clear the cache between plan requests.
		//
		// In the case of concurrent plans, this does mean we may stomp on the others cache. Which
		// would mitigate the benefit of the cache, but should not be a correctness problem.
		resetMeshCaches(ctx, logger, request)
		span.End()
	}()

	if err := request.validatePlanRequest(); err != nil {
		return nil, meta, err
	}
	logger.CDebugf(ctx, "constraint specs for this step: %v", request.Constraints)
	logger.CDebugf(ctx, "motion config for this step: %v", request.PlannerOptions)
	logger.CDebugf(ctx, "start position: %v", request.StartState.structuredConfiguration)

	if request.PlannerOptions == nil {
		request.PlannerOptions = NewBasicPlannerOptions()
	}

	// Theoretically, a plan could be made between two poses, by running IK on both the start and end poses to create sets of seed and
	// goal configurations. However, the blocker here is the lack of a "known good" configuration used to determine which obstacles
	// are allowed to collide with one another.
	if request.StartState.structuredConfiguration == nil {
		return nil, meta, errors.New("must populate start state configuration")
	}

	sfPlanner, err := newPlanManager(ctx, logger, request, meta)
	if err != nil {
		return nil, meta, err
	}

	trajAsInps, goalsProcessed, err := sfPlanner.planMultiWaypoint(ctx)
	if err != nil {
		if request.PlannerOptions.ReturnPartialPlan {
			meta.Partial = true
			meta.PartialError = err
			logger.Infof("returning partial plan, error: %v", err)
		} else {
			return nil, meta, err
		}
	}

	meta.GoalsProcessed = goalsProcessed

	t, err := motionplan.NewSimplePlanFromTrajectory(trajAsInps, request.FrameSystem)
	if err != nil {
		return nil, meta, err
	}

	return t, meta, nil
}

// PlanMotionSequence plans a series of requests that are meant to run back to back. Only the first
// request's StartState is used as given. Every later request starts from the configuration the
// previous plan ended in, for the frames that have degrees of freedom. The frame system, obstacles,
// constraints and planner options of each request are its own, but the frame systems are expected
// to describe the same frames with degrees of freedom.
//
// The StartState of every request after the first is mutated in place. Whatever the caller put
// there is discarded, so there are no requirements on it. It is set to the end of the previous plan
// just before the request is planned, and again each time the request is re-planned. Callers should
// not rely on its value afterwards. As with PlanMotion, a nil PlannerOptions is also filled in.
// Requests also accumulate internal state while backtracking, so they should not be reused for
// another call.
//
// If a request cannot be planned from where the previous plan ended, an earlier request is planned
// again with the end configuration that led to the failure ruled out, and the sequence continues
// from the new answer. Each start state of a request gets 3 attempts, or as many as
// WithBacktrackBranchFactor sets, before the search backs up another request. The cost of that
// search grows exponentially with how far it has to back up.
//
// PlanMotionSequence returns plans and metas, each with one entry per request. When it returns an
// error or a partial plan, entries for the requests after the last good one are nil if never
// planned, or hold whatever an abandoned attempt produced. A PlanMeta sums the numeric values over
// every attempt at its request, and its BacktrackCounter says how many times the request was
// re-planned. An error means no combination of solutions was found. A request with
// ReturnPartialPlan set that only partially plans ends the sequence early with its partial plan and
// no error, as PlanMotion would report it.
func PlanMotionSequence(
	ctx context.Context,
	parentLogger logging.Logger,
	requests []*PlanRequest,
	opts ...SequenceOption,
) ([]motionplan.Plan, []*PlanMeta, error) {
	cfg := sequenceConfig{backtrackBranchFactor: defaultBacktrackBranchFactor}
	for _, opt := range opts {
		opt(&cfg)
	}

	ctx, span := trace.StartSpan(ctx, "PlanMotionSequence")
	defer span.End()

	for _, followupRequest := range requests[1:] {
		followupRequest.StartState = nil
	}

	logger := parentLogger.Sublogger("mp")
	plans := make([]motionplan.Plan, len(requests))
	metas := make([]*PlanMeta, len(requests))

	// requestIdx is managed by the success/failure of individual plan solve attempts.
	for requestIdx := 0; requestIdx < len(requests); {
		request := requests[requestIdx]
		if request.rejectedSolutions == nil {
			request.rejectedSolutions = make(map[*PlanState][]referenceframe.FrameSystemInputs)
		}

		if requestIdx > 0 && request.StartState == nil {
			priorTraj := plans[requestIdx-1].Trajectory()
			request.StartState = NewPlanState(nil, priorTraj[len(priorTraj)-1])
		}
		parentLogger.Info("Working on request:", requestIdx, "Substituted start state:", request.StartState.Configuration())

		plan, meta, err := PlanMotion(ctx, logger, request)
		if metas[requestIdx] != nil {
			// This was a backtracked execution. Increment the counter
			metas[requestIdx].BacktrackCounter++
		}

		// metas[requestIdx] will always be non-nil at this point.
		metas[requestIdx] = metas[requestIdx].Merge(*meta)
		if err == nil {
			plans[requestIdx] = plan
			if meta.Partial {
				logger.Warnf("Unintended use of PlanMotionSequence. Plan request only partially planned. Stopping here. Plan: %v/%v",
					requestIdx+1, len(requests))
				return plans, metas, nil
			}

			// We have a (so far) working solution. Move onto the next request that follows from
			// this solution.
			requestIdx++
			continue
		}

		for {
			// If we errored on some request using a solution from some prior request, let's re-solve
			// the earlier request in a way that forces a new answer -> start state.
			//
			// If we've rejected a bunch of solutions from the prior request, let's backtrack a step
			// further.
			rejectedSolution := request.StartState.Configuration()

			// Playing tricks to make the map lookup work on PlanState pointers. As opposed to using
			// a logical value that represents the "exact" slice of float64 inputs. Nil this out
			// only when we've given up on using the StartState value for all future guesses.
			request.StartState = nil

			// Side-effect for the outer loop.
			requestIdx--
			if requestIdx < 0 {
				return plans, metas, fmt.Errorf("Failed to solve the first plan. Rejected solutions before giving up: %v",
					len(request.rejectedSolutions[request.StartState]))
			}

			// Just reusing the variable for convenience. It's always reset on loop entry anyways.
			request = requests[requestIdx]

			// Force a follow-up solve of the request to avoid a solution that we've determined is a
			// deadend.
			request.rejectedSolutions[request.StartState] = append(
				request.rejectedSolutions[request.StartState], rejectedSolution,
			)
			if len(request.rejectedSolutions[request.StartState]) < cfg.backtrackBranchFactor {
				// If the start state for this plan has only been tried a few times, give it another
				// shot. Continue back onto the outer-loop.
				logger.Infof("Backtracking. Adding rejected solution. Rerunning plan: %v", requestIdx)
				break
			}

			logger.Infof("Backtracking further. Hit maximum retries from start state. Plan: %v StartState: %v",
				requestIdx, request.StartState)
		}
	}

	return plans, metas, nil
}

// defaultBacktrackBranchFactor is how many endings of a request PlanMotionSequence tries from one
// start state before backtracking further.
const defaultBacktrackBranchFactor = 3

type sequenceConfig struct {
	backtrackBranchFactor int
}

// SequenceOption configures PlanMotionSequence.
type SequenceOption func(*sequenceConfig)

// WithBacktrackBranchFactor sets how many endings of a request PlanMotionSequence tries from one
// start state before backtracking further. Values below 1 are treated as 1.
func WithBacktrackBranchFactor(n int) SequenceOption {
	return func(cfg *sequenceConfig) {
		cfg.backtrackBranchFactor = max(n, 1)
	}
}

// resetMeshCaches walks the robot link and world obstacle geometries reachable
// from the plan request and clears any mesh-level collision caches that built
// up during planning. The caches are an optimization, so a best-effort pass is
// fine — failures here are logged and swallowed rather than surfaced as a
// planning error.
func resetMeshCaches(ctx context.Context, logger logging.Logger, request *PlanRequest) {
	visit := func(geometry spatialmath.Geometry) {
		if mesh, ok := geometry.(*spatialmath.Mesh); ok {
			mesh.ResetCache()
		}
	}

	if request.StartState != nil && request.StartState.structuredConfiguration != nil {
		frameSystemGeometries, err := referenceframe.FrameSystemGeometries(
			request.FrameSystem,
			request.StartState.structuredConfiguration,
		)
		if err != nil {
			logger.CDebugf(ctx, "resetMeshCaches: skipping robot geometries: %v", err)
			// Not expected to happen, but we still want to try walking the world state. Rely on
			// ranging over a nil slice to be a no-op.
		}

		for _, geometriesInFrame := range frameSystemGeometries {
			for _, geometry := range geometriesInFrame.Geometries() {
				visit(geometry)
			}
		}
	}

	if request.ObstaclesInWorldFrame != nil {
		// Right now, the ObstaclesInWorldFrame is getting entirely thrown away anyways. It's always
		// a part of the user request that is about to go out of scope. In the future, this may
		// include geometries saved in the `WorldStateStore` service.
		for _, geometry := range request.ObstaclesInWorldFrame.Geometries() {
			visit(geometry)
		}
	}
}

var defaultArmPlannerOptions = &motionplan.Constraints{
	LinearConstraint: []motionplan.LinearConstraint{},
}

// MoveArm is a helper function to abstract away movement for general arms.
func MoveArm(ctx context.Context, logger logging.Logger, a arm.Arm, dst spatialmath.Pose) error {
	inputs, err := a.CurrentInputs(ctx)
	if err != nil {
		return err
	}

	model, err := a.Kinematics(ctx)
	if err != nil {
		return err
	}
	if _, err = model.Transform(inputs); err != nil {
		return err
	}

	plan, err := PlanFrameMotion(ctx, logger, dst, model, inputs, defaultArmPlannerOptions, nil)
	if err != nil {
		return err
	}
	return a.MoveThroughJointPositions(ctx, plan, nil, nil)
}

// ReadRequestFromFile reads a PlanRequest from a json file. This method is compatible with legacy
// PlanRequests containing a `world_state` key as well as modern ones that simply take in
// `obstacles_in_world_frame`. Files may contain a second JSON object (a response) after the
// request; this function decodes only the first object.
func ReadRequestFromFile(fileName string) (*PlanRequest, error) {
	req, _, err := ReadRequestAndResponseFromFile(fileName)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// WriteToFile write a request to a .json file.
func (req *PlanRequest) WriteToFile(fileName string) error {
	file, err := os.OpenFile(filepath.Clean(fileName), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(file.Close)

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	_, err = file.Write(data)
	if err != nil {
		return err
	}

	return nil
}

// ReadRequestAndResponseFromFile reads a PlanRequest and optionally a motionplan.Plan from a JSON
// file written by WriteRequestAndResponseToFile. The returned plan is nil if the file contains only
// a request.
func ReadRequestAndResponseFromFile(fileName string) (*PlanRequest, motionplan.Plan, error) {
	f, err := os.Open(fileName) //nolint:gosec
	if err != nil {
		return nil, nil, err
	}
	defer utils.UncheckedErrorFunc(f.Close)

	return RequestFromReader(f)
}

// RequestFromReader is the same as ReadRequestAndResponseFromFile but takes a generic `io.Reader`
// as input.
func RequestFromReader(reader io.Reader) (*PlanRequest, motionplan.Plan, error) {
	decoder := json.NewDecoder(reader)

	// We first decode the file into a raw json structure. This is because we have best effort
	// support for reading different versions of request files. The current version of the
	// `PlanRequest` object may not map perfectly to some historical serialization.
	var raw json.RawMessage
	var err error
	if err = decoder.Decode(&raw); err != nil {
		return nil, nil, err
	}

	var probe map[string]json.RawMessage
	if err = json.Unmarshal(raw, &probe); err != nil {
		return nil, nil, err
	}

	// We've removed world state from the plan request object. Instead forcing callers to merge
	// world state transforms into the `FrameSystem` member itself. And world state obstacles are
	// now passed in directly.
	//
	// PlannerOptions is pre-seeded with defaults so a saved options object that omits a field
	// merges into the default rather than the type's zero value. A request captured before a
	// field existed (or with the key stripped) must replay with the same effective options
	// production would use - most acutely collision_buffer_mm, where a decoded 0 flips collision
	// verdicts and can turn a milliseconds plan into a timeout.
	req := &PlanRequest{PlannerOptions: NewBasicPlannerOptions()}
	if _, hasWorldState := probe["world_state"]; hasWorldState {
		// Legacy format, parse as a `PlanRequestWithWorldState` and have that "upgrade" to a modern
		// `PlanRequest`.
		legacy := &PlanRequestWithWorldState{PlannerOptions: NewBasicPlannerOptions()}
		if err = json.Unmarshal(raw, legacy); err != nil {
			return nil, nil, err
		}

		// Dan: We explicitly ignore world state transforms when "upgrading" from this legacy
		// request file. This was introduced because of a bug where world state transforms weren't
		// being used at all. So this preserves that behavior. But maybe it's better to do merging
		// as that was always the intent?
		req, err = legacy.ToPlanRequestWorldStateTransformsIgnored()
		if err != nil {
			return nil, nil, err
		}
	} else {
		if err = json.Unmarshal(raw, req); err != nil {
			return nil, nil, err
		}
	}

	// RSDK-14172: Plan requests can have a nil PlannerOptions. PlanMotion (currently) substitutes
	// in the `NewBasicPlannerOptions`.
	if req.PlannerOptions == nil {
		req.PlannerOptions = NewBasicPlannerOptions()
	}

	plan := &motionplan.SimplePlan{}
	if err = decoder.Decode(plan); err != nil {
		if errors.Is(err, io.EOF) {
			return req, nil, nil
		}
		return nil, nil, err
	}

	return req, plan, nil
}

// WriteRequestAndResponseToFile writes the `req` as one JSON object followed by `resp` as a second
// JSON object. This is backwards compatible with existing `ReadRequestFromFile`
// implementations. Also see `ReadRequestAndResponseFromFile` to pull out both sets of
// information. `resp` can be nil. Useful for cases where the request resulted in a motion plan
// error.
func (req *PlanRequest) WriteRequestAndResponseToFile(filename string, resp motionplan.Plan) error {
	file, err := os.OpenFile(filepath.Clean(filename), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(file.Close)

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	_, err = file.Write(data)
	if err != nil {
		return err
	}

	if resp != nil {
		data, err = json.Marshal(resp)
		if err != nil {
			return err
		}

		_, err = file.Write(data)
		if err != nil {
			return err
		}
	}

	return nil
}
