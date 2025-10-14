package armplanning

import (
	"context"
	"math"
	"math/rand"

	"go.opencensus.io/trace"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

type planContext struct {
	fs  *referenceframe.FrameSystem
	lfs *linearizedFrameSystem

	boundingRegions []spatialmath.Geometry

	configurationDistanceFunc motionplan.SegmentFSMetric
	planOpts                  *PlannerOptions
	request                   *PlanRequest

	randseed *rand.Rand

	planMeta *PlanMeta
	logger   logging.Logger
}

func newPlanContext(ctx context.Context, logger logging.Logger, request *PlanRequest, meta *PlanMeta) (*planContext, error) {
	ctx, span := trace.StartSpan(ctx, "newPlanContext")
	defer span.End()
	pc := &planContext{
		fs:                        request.FrameSystem,
		configurationDistanceFunc: motionplan.GetConfigurationDistanceFunc(request.PlannerOptions.ConfigurationDistanceMetric),
		planOpts:                  request.PlannerOptions,
		request:                   request,
		randseed:                  rand.New(rand.NewSource(int64(request.PlannerOptions.RandomSeed))), //nolint:gosec
		planMeta:                  meta,
		logger:                    logger,
	}

	var err error
	pc.lfs, err = newLinearizedFrameSystem(pc.fs)
	if err != nil {
		return nil, err
	}

	pc.boundingRegions, err = referenceframe.NewGeometriesFromProto(request.BoundingRegions)
	if err != nil {
		return nil, err
	}

	return pc, nil
}

// linearize the goal metric for use with solvers.
// Since our solvers operate on arrays of floats, there needs to be a way to map bidirectionally between the framesystem configuration
// of FrameSystemInputs and the []float64 that the solver expects. This is that mapping.
func (pc *planContext) linearizeFSmetric(metric motionplan.StateFSMetric) func([]float64) float64 {
	return func(query []float64) float64 {
		inputs, err := pc.lfs.sliceToMap(query)
		if err != nil {
			return math.Inf(1)
		}
		return metric(&motionplan.StateFS{Configuration: inputs, FS: pc.fs})
	}
}

type planSegmentContext struct {
	pc *planContext

	start    referenceframe.FrameSystemInputs
	origGoal referenceframe.FrameSystemPoses // goals are defined in frames willy nilly
	goal     referenceframe.FrameSystemPoses // all in world

	startPoses referenceframe.FrameSystemPoses

	motionChains *motionChains
	checker      *motionplan.ConstraintChecker
}

func newPlanSegmentContext(ctx context.Context, pc *planContext, start referenceframe.FrameSystemInputs,
	goal referenceframe.FrameSystemPoses,
) (*planSegmentContext, error) {
	ctx, span := trace.StartSpan(ctx, "newPlanSegmentContext")
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

	// TODO: this is duplicated work as it's also done in motionplan.NewConstraintChecker
	frameSystemGeometries, err := referenceframe.FrameSystemGeometries(pc.fs, start)
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
		pc.boundingRegions,
		false,
	)
	if err != nil {
		return nil, err
	}

	return psc, nil
}

func (psc *planSegmentContext) checkPath(ctx context.Context, start, end referenceframe.FrameSystemInputs) error {
	_, span := trace.StartSpan(ctx, "checkPath")
	defer span.End()
	_, err := psc.checker.CheckSegmentAndStateValidityFS(
		&motionplan.SegmentFS{
			StartConfiguration: start,
			EndConfiguration:   end,
			FS:                 psc.pc.fs,
		},
		psc.pc.planOpts.Resolution,
	)
	return err
}

func (psc *planSegmentContext) checkInputs(inputs referenceframe.FrameSystemInputs) bool {
	return psc.checker.CheckStateFSConstraints(&motionplan.StateFS{
		Configuration: inputs,
		FS:            psc.pc.fs,
	}) == nil
}

func translateGoalsToWorldPosition(
	fs *referenceframe.FrameSystem,
	start referenceframe.FrameSystemInputs,
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
