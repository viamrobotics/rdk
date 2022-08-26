package motionplan

const (
	// Number of planner iterations before giving up.
	defaultPlanIter = 2000
)

type rrtOptions struct {
	// Number of planner iterations before giving up.
	PlanIter int `json:"plan_iter"`

	// Contains constraints, IK solving params, etc
	planOpts *PlannerOptions
}

func newRRTOptions(planOpts *PlannerOptions) *rrtOptions {
	return &rrtOptions{
		PlanIter: defaultPlanIter,
		planOpts: planOpts,
	}
}
