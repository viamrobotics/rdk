package motionplan

import "errors"

func newIKError() error {
	return errors.New("unable to solve for position")
}

func newPlannerFailedError() error {
	return errors.New("motion planner failed to find path")
}

func newPlannerOptionsError() error {
	return errors.New("PlannerOptions are required but have not been specified")
}
