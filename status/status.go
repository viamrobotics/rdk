// Package status define the status structures of a robot.
package status

import (
	"context"

	"github.com/go-errors/errors"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/robot"
)

// Create constructs a new up to date status from the given robot.
// The operation can take time and be expensive, so it can be cancelled by the
// given context.
func Create(ctx context.Context, r robot.Robot) (*pb.Status, error) {
	var err error
	var status pb.Status

	// manually refresh all remotes to get an up to date status
	for _, name := range r.RemoteNames() {
		remote, ok := r.RemoteByName(name)
		if !ok {
			continue
		}
		if refresher, ok := remote.(robot.Refresher); ok {
			if err := refresher.Refresh(ctx); err != nil {
				return nil, errors.Errorf("error refreshing remote %q: %w", name, err)
			}
		}
	}

	if names := r.ArmNames(); len(names) != 0 {
		status.Arms = make(map[string]*pb.ArmStatus, len(names))
		for _, name := range names {
			arm, ok := r.ArmByName(name)
			if !ok {
				continue
			}
			armStatus := &pb.ArmStatus{}

			armStatus.GridPosition, err = arm.CurrentPosition(ctx)
			if err != nil {
				return nil, err
			}
			armStatus.JointPositions, err = arm.CurrentJointPositions(ctx)
			if err != nil {
				return nil, err
			}

			status.Arms[name] = armStatus
		}
	}

	if names := r.GripperNames(); len(names) != 0 {
		status.Grippers = make(map[string]bool, len(names))
		for _, name := range names {
			status.Grippers[name] = true
		}
	}

	if names := r.BaseNames(); len(names) != 0 {
		status.Bases = make(map[string]bool, len(names))
		for _, name := range names {
			status.Bases[name] = true
		}
	}

	if names := r.BoardNames(); len(names) != 0 {
		status.Boards = make(map[string]*pb.BoardStatus, len(names))
		for _, name := range names {
			board, ok := r.BoardByName(name)
			if !ok {
				continue
			}
			boardStatus, err := board.Status(ctx)
			if err != nil {
				return nil, err
			}
			status.Boards[name] = boardStatus
		}
	}

	if names := r.CameraNames(); len(names) != 0 {
		status.Cameras = make(map[string]bool, len(names))
		for _, name := range names {
			status.Cameras[name] = true
		}
	}

	if names := r.LidarNames(); len(names) != 0 {
		status.Lidars = make(map[string]bool, len(names))
		for _, name := range names {
			status.Lidars[name] = true
		}
	}

	if names := r.SensorNames(); len(names) != 0 {
		status.Sensors = make(map[string]*pb.SensorStatus, len(names))
		for _, name := range names {
			sensorDevice, ok := r.SensorByName(name)
			if !ok {
				continue
			}
			status.Sensors[name] = &pb.SensorStatus{
				Type: string(sensorDevice.Desc().Type),
			}
		}
	}

	if names := r.FunctionNames(); len(names) != 0 {
		status.Functions = make(map[string]bool, len(names))
		for _, name := range names {
			status.Functions[name] = true
		}
	}

	return &status, nil
}
