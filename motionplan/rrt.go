package motionplan

const (
	// Number of planner iterations before giving up.
	defaultPlanIter = 2000
	
	defaultTimeout = 25.0
)

type rrtOptions struct {
	// Number of seconds before terminating planner
	Timeout float64 `json:"timeout"`
	
	// Number of planner iterations before giving up.
	PlanIter int `json:"plan_iter"`

	// Contains constraints, IK solving params, etc
	planOpts *PlannerOptions
}

func newRRTOptions(planOpts *PlannerOptions) *rrtOptions {
	return &rrtOptions{
		Timeout: defaultTimeout,
		PlanIter: defaultPlanIter,
		planOpts: planOpts,
	}
}
