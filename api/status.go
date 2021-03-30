package api

import (
	pb "go.viam.com/robotcore/proto/api/v1"
)

func CreateStatus(r Robot) (*pb.Status, error) {
	var err error
	s := &pb.Status{
		Arms:     map[string]*pb.ArmStatus{},
		Bases:    map[string]bool{},
		Grippers: map[string]bool{},
		Boards:   map[string]*pb.BoardStatus{},
	}

	for _, name := range r.ArmNames() {
		arm := r.ArmByName(name)
		as := &pb.ArmStatus{}

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
