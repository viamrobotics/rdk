package armplanning

import (
	"context"
	"math"
	"math/rand"
	"strings"

	"go.opencensus.io/trace"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

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
	)
	if err != nil {
		return nil, err
	}

	return psc, nil
}

func (psc *planSegmentContext) checkPath(ctx context.Context, start, end *referenceframe.LinearInputs) error {
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
