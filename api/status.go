package api

type ArmStatus struct {
	GridPosition   ArmPosition
	JointPositions JointPositions
}

type Status struct {
	Arms     map[string]ArmStatus
	Grippers map[string]bool // TODO(erh): not sure what this should be, but ok for now
}

func CreateStatus(r Robot) (Status, error) {
	var err error
	s := Status{
		Arms:     map[string]ArmStatus{},
		Grippers: map[string]bool{},
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

	for _, name := range r.GripperNames() {
		s.Grippers[name] = true
	}

	return s, nil
}
