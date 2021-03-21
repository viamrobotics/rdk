package api

import (
	"go.viam.com/robotcore/board"
)

type ArmStatus struct {
	GridPosition   ArmPosition
	JointPositions JointPositions
}

type Status struct {
	Arms     map[string]ArmStatus
	Bases    map[string]bool // TODO(erh): not sure what this should be, but ok for now
	Grippers map[string]bool // TODO(erh): not sure what this should be, but ok for now
	Boards   map[string]board.Status
}

func CreateStatus(r Robot) (Status, error) {
	var err error
	s := Status{
		Arms:     map[string]ArmStatus{},
		Bases:    map[string]bool{},
		Grippers: map[string]bool{},
		Boards:   map[string]board.Status{},
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

	for _, name := range r.BaseNames() {
		s.Bases[name] = true
	}

	for _, name := range r.BoardNames() {
		s.Boards[name], err = r.BoardByName(name).Status()
		if err != nil {
			return s, err
		}
	}

	return s, nil
}
