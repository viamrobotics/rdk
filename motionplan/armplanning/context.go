package armplanning

import (
	"context"
	"encoding/binary"
	"hash/fnv"
	"math"
	"math/rand"
	"strings"

	"go.viam.com/utils/trace"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

type PlanContext struct {
	fs  *referenceframe.FrameSystem
	lis *referenceframe.LinearInputsSchema

	movableFrames []string

	configurationDistanceFunc motionplan.SegmentFSMetric
	planOpts                  *PlannerOptions
	request                   *PlanRequest

	randseed *rand.Rand

	planMeta *PlanMeta
	logger   logging.Logger

	// collisionCache is the per-plan temporal-coherence cache for collision
	// checking. Threaded down through every ConstraintChecker built for this
	// plan and into the mesh BVH layer via the CollisionWitnessCache interface.
	// One cache per plan; lives for the plan's lifetime so witnesses + edge
	// memoization accumulate.
	collisionCache *motionplan.CollisionCache
}

func NewPlanContext(ctx context.Context, logger logging.Logger, request *PlanRequest, meta *PlanMeta) (*PlanContext, error) {
	_, span := trace.StartSpan(ctx, "NewPlanContext")
	defer span.End()
	meta.CollectSolutionDiagnostics = request.PlannerOptions.CollectSolutionDiagnostics
	pc := &PlanContext{
		fs:                        request.FrameSystem,
		configurationDistanceFunc: motionplan.GetConfigurationDistanceFunc(request.PlannerOptions.ConfigurationDistanceMetric),
		planOpts:                  request.PlannerOptions,
		request:                   request,
		randseed:                  rand.New(rand.NewSource(int64(request.PlannerOptions.RandomSeed))), //nolint:gosec
		planMeta:                  meta,
		logger:                    logger,
		collisionCache:            motionplan.NewCollisionCache(),
	}

	var err error
	pc.lis, err = request.StartState.LinearConfiguration().GetSchema(pc.fs)
	if err != nil {
		return nil, err
	}

	for _, fn := range pc.fs.FrameNames() {
		f := pc.fs.Frame(fn)
		if len(f.DoF()) > 0 {
			pc.movableFrames = append(pc.movableFrames, fn)
		}
	}

	return pc, nil
}

func (pc *PlanContext) linearizeFSmetric(metric motionplan.StateFSMetric) ik.CostFunc {
	return func(ctx context.Context, linearizedInputs []float64) float64 {
		conf, err := pc.lis.FloatsToInputs(linearizedInputs)
		if err != nil {
			return math.Inf(1)
		}

		return metric(&motionplan.StateFS{
			Configuration: conf,
			FS:            pc.fs,
		})
	}
}

type PlanSegmentContext struct {
	pc *PlanContext

	start    *referenceframe.LinearInputs
	origGoal referenceframe.FrameSystemPoses // goals are defined in frames willy nilly
	goal     referenceframe.FrameSystemPoses // all in world

	startPoses referenceframe.FrameSystemPoses

	motionChains *motionChains
	checker      *motionplan.ConstraintChecker
}

func NewPlanSegmentContext(ctx context.Context, pc *PlanContext, start *referenceframe.LinearInputs,
	goal referenceframe.FrameSystemPoses,
) (*PlanSegmentContext, error) {
	_, span := trace.StartSpan(ctx, "NewPlanSegmentContext")
	defer span.End()
	psc := &PlanSegmentContext{
		pc:       pc,
		start:    start,
		origGoal: goal,
	}

	var err error
	// Translate user-stated goals into the world reference frame. Consider the case where one
	// desires to move the gripper forward 100mm:
	//
	// `Move("gripper", PoseInFrame("gripper", {X: 100}))`.
	//
	// We must evaluate the goal position before moving, otherwise we'll be trying to hit a forever
	// moving target.
	psc.goal, err = translateGoalsToWorldPosition(pc.fs, psc.start, psc.origGoal)
	if err != nil {
		return nil, err
	}

	psc.startPoses, err = start.ComputePoses(pc.fs)
	if err != nil {
		return nil, err
	}

	psc.motionChains, err = motionChainsFromPlanState(pc.fs, goal)
	if err != nil {
		return nil, err
	}

	// TODO: this is duplicated work as it's also done in motionplan.NewConstraintChecker
	frameSystemGeometries, err := referenceframe.FrameSystemGeometries(pc.fs, start.ToFrameSystemInputs())
	if err != nil {
		return nil, err
	}

	movingRobotGeometries, staticRobotGeometries := psc.motionChains.geometries(pc.fs, frameSystemGeometries)

	psc.checker, err = motionplan.NewConstraintChecker(
		pc.planOpts.CollisionBufferMM,
		pc.request.Constraints,
		psc.startPoses,
		goal,
		pc.fs,
		movingRobotGeometries, staticRobotGeometries,
		start,
		pc.request.WorldState,
		pc.logger.Sublogger("constraint"),
		pc.collisionCache,
	)
	if err != nil {
		return nil, err
	}

	return psc, nil
}

// checkPath returns an error if the interpolation between `start` and `end` violate a constraint
// (e.g: we calculcate there will be a collision). If there is an error and `outPath` is non-nil,
// `outPath` will be populated with more detailed information.
func (psc *PlanSegmentContext) checkPath(
	ctx context.Context, start, end *referenceframe.LinearInputs, checkFinal bool, outPath *pathFeedback,
) error {
	ctx, span := trace.StartSpan(ctx, "checkPath")
	defer span.End()

	// Edge-result memoization: RRT-Connect rewire and path smoothing re-check
	// the same (start, end) edges constantly. Skip the full interpolated sweep
	// when the cache has already verified this edge collision-free under the
	// same checkFinal setting. Negative results aren't cached — they depend on
	// the failure step, which depends on resolution and buffer.
	cache := psc.pc.collisionCache
	hashA := hashLinearInputs(start)
	hashB := hashLinearInputs(end)
	if cache != nil && hashA != 0 && hashB != 0 {
		if isClear, ok := cache.LookupEdgeResult(hashA, hashB); ok && isClear {
			return nil
		}
	}

	validSegment, err := psc.checker.CheckStateConstraintsAcrossSegmentFS(
		ctx,
		&motionplan.SegmentFS{
			StartConfiguration: start,
			EndConfiguration:   end,
			FS:                 psc.pc.fs,
		},
		psc.pc.planOpts.Resolution,
		checkFinal,
	)
	if err == nil && cache != nil && hashA != 0 && hashB != 0 {
		cache.StoreEdgeResult(hashA, hashB, true)
	}

	if err != nil && outPath != nil {
		*outPath = pathFeedback{
			IsObstacleCollision: strings.Contains(err.Error(), motionplan.ObstacleConstraintDescription) ||
				strings.Contains(err.Error(), motionplan.RobotCollisionConstraintDescription),
			LastGoodInputs: validSegment.EndConfiguration,
		}
	}
	return err
}

// hashLinearInputs computes a deterministic FNV-1a hash over the float values
// in a LinearInputs, used as a cache key for edge memoization. Returns 0 for
// nil inputs (cache layer treats 0 as "no key").
func hashLinearInputs(li *referenceframe.LinearInputs) uint64 {
	if li == nil {
		return 0
	}
	floats := li.GetLinearizedInputs()
	if len(floats) == 0 {
		return 0
	}
	h := fnv.New64a()
	var buf [8]byte
	for _, f := range floats {
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(f))
		_, _ = h.Write(buf[:])
	}
	return h.Sum64()
}

func translateGoalsToWorldPosition(
	fs *referenceframe.FrameSystem,
	start *referenceframe.LinearInputs,
	goal referenceframe.FrameSystemPoses,
) (referenceframe.FrameSystemPoses, error) {
	alteredGoals := referenceframe.FrameSystemPoses{}
	for f, pif := range goal {
		tf, err := fs.Transform(start, pif, referenceframe.World)
		if err != nil {
			return nil, err
		}

		alteredGoals[f] = tf.(*referenceframe.PoseInFrame)
	}
	return alteredGoals, nil
}

func (pc *PlanContext) isFatalCollision(err error) bool {
	s := err.Error()
	if strings.Contains(s, "obstacle constraint: violation") {
		hasMovingFrame := false
		for _, f := range pc.movableFrames {
			if strings.Contains(s, f) {
				hasMovingFrame = true
				break
			}
		}
		return !hasMovingFrame
	}
	return false
}
