package motionplan

import "errors"

var errIKSolve = errors.New("no IK solutions, check if goal outside workspace")

var errPlannerFailed = errors.New("motion planner failed to find path")

var errNoPlannerOptions = errors.New("PlannerOptions are required but have not been specified")
