package motionplan

import "errors"

var errIKSolve = errors.New("unable to solve for position")

var errPlannerFailed = errors.New("motion planner failed to find path")

var errNoPlannerOptions = errors.New("PlannerOptions are required but have not been specified")
