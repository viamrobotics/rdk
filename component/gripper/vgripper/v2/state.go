package vgripper

// GripperState describes the action the gripper is performing.
type GripperState int32

const (
	gripperStateUnspecified              GripperState = 0
	gripperStateCalibrating              GripperState = 1
	gripperStateOpening                  GripperState = 2
	gripperStateGrabbing                 GripperState = 3
	gripperStateIdle                     GripperState = 4
	gripperStateAntiSlipForceControlling GripperState = 5
)

// Enum value maps for DirectionRelative.
// TODO: write tests for this to make sure that all conversions are a closed loop;
// to make sure human error doesn't happen here
var (
	gripperStateName = map[GripperState]string{
		0: "gripperStateUnspecified",
		1: "gripperStateCalibrating",
		2: "gripperStateOpening",
		3: "gripperStateGrabbing",
		4: "gripperStateIdle",
		5: "gripperStateAntiSlipForceControlling",
	}
)
