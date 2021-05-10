package api

import (
	"context"
	"fmt"

	pb "go.viam.com/robotcore/proto/api/v1"
)

// CreateStatus constructs a new up to date status from the given robot.
// The operation can take time and be expensive, so it can be cancelled by the
// given context.
func CreateStatus(ctx context.Context, r Robot) (*pb.Status, error) {
	var err error
	var status pb.Status

	// manually refresh all remotes to get an up to date status
	for _, name := range r.RemoteNames() {
		remote := r.RemoteByName(name)
		if err := remote.Refresh(ctx); err != nil {
			return nil, fmt.Errorf("error refreshing remote %q: %w", name, err)
		}
	}

	if names := r.ArmNames(); len(names) != 0 {
		status.Arms = make(map[string]*pb.ArmStatus, len(names))
		for _, name := range names {
			arm := r.ArmByName(name)
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
			boardStatus, err := r.BoardByName(name).Status(ctx)
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

	if names := r.LidarDeviceNames(); len(names) != 0 {
		status.LidarDevices = make(map[string]bool, len(names))
		for _, name := range names {
			status.LidarDevices[name] = true
		}
	}

	if names := r.SensorNames(); len(names) != 0 {
		status.Sensors = make(map[string]*pb.SensorStatus, len(names))
		for _, name := range names {
			sensorDevice := r.SensorByName(name)
			status.Sensors[name] = &pb.SensorStatus{
				Type: string(sensorDevice.Desc().Type),
			}
		}
	}

	return &status, nil
}
