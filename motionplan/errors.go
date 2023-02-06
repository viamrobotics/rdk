package motionplan

import "errors"

var errIKSolve = errors.New("zero IK solutions produced, goal positions appears to be physically unreachable")

var errPlannerFailed = errors.New("motion planner failed to find path")

var errNoPlannerOptions = errors.New("PlannerOptions are required but have not been specified")
