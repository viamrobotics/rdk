package api

type ArmStatus struct {
	GridPosition   ArmPosition
	JointPositions JointPositions
}

type Status struct {
	Arms map[string]ArmStatus
}

func CreateStatus(r Robot) (Status, error) {
	var err error
	s := Status{
		Arms: map[string]ArmStatus{},
	}

	for _, name := range r.ArmNames() {
		arm := r.ArmByName(name)
		as := ArmStatus{}

		as.GridPosition, err = arm.CurrentPosition()
		if err != nil {
			return s, err
		}
		as.JointPositions, err = arm.CurrentJointPositions()
		if err != nil {
			return s, err
		}

		s.Arms[name] = as
	}

	return s, nil
}
