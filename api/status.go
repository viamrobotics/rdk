package api

import (
	"context"

	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/sensor/compass"
)

func CreateStatus(ctx context.Context, r Robot) (*pb.Status, error) {
	var err error
	s := &pb.Status{
		Arms:         map[string]*pb.ArmStatus{},
		Bases:        map[string]bool{},
		Grippers:     map[string]bool{},
		Boards:       map[string]*pb.BoardStatus{},
		Cameras:      map[string]bool{},
		LidarDevices: map[string]bool{},
		Sensors:      map[string]*pb.SensorStatus{},
	}

	for _, name := range r.ArmNames() {
		arm := r.ArmByName(name)
		as := &pb.ArmStatus{}

		as.GridPosition, err = arm.CurrentPosition(ctx)
		if err != nil {
			return s, err
		}
		as.JointPositions, err = arm.CurrentJointPositions(ctx)
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
		s.Boards[name], err = r.BoardByName(name).Status(ctx)
		if err != nil {
			return s, err
		}
	}

	for _, name := range r.CameraNames() {
		s.Cameras[name] = true
	}

	for _, name := range r.LidarDeviceNames() {
		s.LidarDevices[name] = true
	}

	for _, name := range r.SensorNames() {
		sensorDevice := r.SensorByName(name)
		s.Sensors[name] = &pb.SensorStatus{
			Type: string(GetSensorDeviceType(sensorDevice)),
		}
	}

	return s, nil
}

func GetSensorDeviceType(s sensor.Device) sensor.DeviceType {
	switch s.(type) {
	case compass.Device:
		if _, ok := s.(compass.RelativeDevice); ok {
			return compass.RelativeDeviceType
		}
		return compass.DeviceType
	default:
		return sensor.DeviceType("") // unknown
	}
}
