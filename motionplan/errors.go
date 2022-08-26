package motionplan

import "errors"

func NewIKError() error {
	return errors.New("unable to solve for position")
}

func NewPlannerFailedError() error {
	return errors.New("motion planner failed to find path")
}

func NewPlannerOptionsError() error {
	return errors.New("PlannerOptions are required but have not been specified")
}
