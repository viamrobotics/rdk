package vgripper

// gripperState describes the action the gripper is performing.
type gripperState int32

const (
	gripperStateUnspecified              gripperState = 0
	gripperStateCalibrating              gripperState = 1
	gripperStateOpening                  gripperState = 2
	gripperStateGrabbing                 gripperState = 3
	gripperStateIdle                     gripperState = 4
	gripperStateAntiSlipForceControlling gripperState = 5
)

// Enum value maps for DirectionRelative.
// TODO: write tests for this to make sure that all conversions are a closed loop;
// to make sure human error doesn't happen here
var (
	gripperStateName = map[gripperState]string{
		0: "gripperStateUnspecified",
		1: "gripperStateCalibrating",
		2: "gripperStateOpening",
		3: "gripperStateGrabbing",
		4: "gripperStateIdle",
		5: "gripperStateAntiSlipForceControlling",
	}
)
