package armplanning

import (
	"context"
	"math"
	"math/rand"
	"sort"
	"strings"

	"go.viam.com/utils/trace"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// collisionConstraintCacheEntry caches collision constraints for a specific set of geometries.
type collisionConstraintCacheEntry struct {
	constraints map[string]motionplan.CollisionConstraintFunc
}

// geometrySignature creates a deterministic key from geometry labels for caching.
func geometrySignature(movingGeoms, staticGeoms []spatialmath.Geometry) string {
	var labels []string
	for _, g := range movingGeoms {
		labels = append(labels, "m:"+g.Label())
	}
	for _, g := range staticGeoms {
		labels = append(labels, "s:"+g.Label())
	}
	sort.Strings(labels)
	return strings.Join(labels, "|")
}

type planContext struct {
	fs  *referenceframe.FrameSystem
	lis *referenceframe.LinearInputsSchema

	movableFrames []string

	configurationDistanceFunc motionplan.SegmentFSMetric
	planOpts                  *PlannerOptions
	request                   *PlanRequest

	randseed *rand.Rand

	planMeta *PlanMeta
	logger   logging.Logger

	// collisionConstraintCache caches collision constraints keyed by geometry signature.
	// This avoids recreating identical collision constraints for each segment in a multi-waypoint plan.
	collisionConstraintCache map[string]*collisionConstraintCacheEntry
}

func newPlanContext(ctx context.Context, logger logging.Logger, request *PlanRequest, meta *PlanMeta) (*planContext, error) {
	_, span := trace.StartSpan(ctx, "newPlanContext")
	defer span.End()
	pc := &planContext{
		fs:                        request.FrameSystem,
		configurationDistanceFunc: motionplan.GetConfigurationDistanceFunc(request.PlannerOptions.ConfigurationDistanceMetric),
		planOpts:                  request.PlannerOptions,
		request:                   request,
		randseed:                  rand.New(rand.NewSource(int64(request.PlannerOptions.RandomSeed))), //nolint:gosec
		planMeta:                  meta,
		logger:                    logger,
		collisionConstraintCache:  make(map[string]*collisionConstraintCacheEntry),
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

func (pc *planContext) linearizeFSmetric(metric motionplan.StateFSMetric) ik.CostFunc {
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

type planSegmentContext struct {
	pc *planContext

	start    *referenceframe.LinearInputs
	origGoal referenceframe.FrameSystemPoses // goals are defined in frames willy nilly
	goal     referenceframe.FrameSystemPoses // all in world

	startPoses referenceframe.FrameSystemPoses

	motionChains *motionChains
	checker      *motionplan.ConstraintChecker
}

// getOrCreateCollisionConstraints returns cached collision constraints for the given geometries,
// or creates and caches new ones if not found.
func (pc *planContext) getOrCreateCollisionConstraints(
	movingGeoms, staticGeoms []spatialmath.Geometry,
	seedMap *referenceframe.LinearInputs,
) (map[string]motionplan.CollisionConstraintFunc, error) {
	cacheKey := geometrySignature(movingGeoms, staticGeoms)

	if cached, ok := pc.collisionConstraintCache[cacheKey]; ok {
		return cached.constraints, nil
	}

	var collisionSpec []motionplan.CollisionSpecification
	if pc.request.Constraints != nil {
		collisionSpec = pc.request.Constraints.CollisionSpecification
	}

	constraints, err := motionplan.NewCollisionConstraints(
		pc.fs,
		movingGeoms,
		staticGeoms,
		pc.request.WorldState,
		collisionSpec,
		seedMap,
		pc.planOpts.CollisionBufferMM,
	)
	if err != nil {
		return nil, err
	}

	pc.collisionConstraintCache[cacheKey] = &collisionConstraintCacheEntry{
		constraints: constraints,
	}

	return constraints, nil
}

func newPlanSegmentContext(ctx context.Context, pc *planContext, start *referenceframe.LinearInputs,
	goal referenceframe.FrameSystemPoses,
) (*planSegmentContext, error) {
	_, span := trace.StartSpan(ctx, "newPlanSegmentContext")
	defer span.End()
	psc := &planSegmentContext{
		pc:       pc,
		start:    start,
		origGoal: goal,
	}

	var err error
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

	frameSystemGeometries, err := referenceframe.FrameSystemGeometries(pc.fs, start.ToFrameSystemInputs())
	if err != nil {
		return nil, err
	}

	movingRobotGeometries, staticRobotGeometries := psc.motionChains.geometries(pc.fs, frameSystemGeometries)

	collisionConstraints, err := pc.getOrCreateCollisionConstraints(movingRobotGeometries, staticRobotGeometries, start)
	if err != nil {
		return nil, err
	}

	topoConstraint, err := motionplan.NewTopoConstraint(pc.fs, start, psc.startPoses, goal, pc.request.Constraints)
	if err != nil {
		return nil, err
	}

	psc.checker = motionplan.NewConstraintChecker(
		collisionConstraints,
		topoConstraint,
		pc.logger.Sublogger("constraint"),
	)

	return psc, nil
}

func (psc *planSegmentContext) checkPath(ctx context.Context, start, end *referenceframe.LinearInputs, checkFinal bool) error {
	ctx, span := trace.StartSpan(ctx, "checkPath")
	defer span.End()
	_, err := psc.checker.CheckStateConstraintsAcrossSegmentFS(
		ctx,
		&motionplan.SegmentFS{
			StartConfiguration: start,
			EndConfiguration:   end,
			FS:                 psc.pc.fs,
		},
		psc.pc.planOpts.Resolution,
		checkFinal,
	)
	return err
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

func (pc *planContext) isFatalCollision(err error) bool {
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
