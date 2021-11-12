package vgripper

// gripperState describes the action the gripper is performing.
type gripperState int32

const (
	gripperState_UNSPECIFIED                 gripperState = 0
	gripperState_CALIBRATING                 gripperState = 1
	gripperState_OPENING                     gripperState = 2
	gripperState_GRABBING                    gripperState = 3
	gripperState_IDLE                        gripperState = 4
	gripperState_ANTI_SLIP_FORCE_CONTROLLING gripperState = 5
	gripperState_EXITING                     gripperState = 6
)

// Enum value maps for DirectionRelative.
// TODO: write tests for this to make sure that all conversions are a closed loop;
// to make sure human error doesn't happen here
var (
	gripperState_name = map[gripperState]string{
		0: "gripperState_UNSPECIFIED",
		1: "gripperState_CALIBRATING",
		2: "gripperState_OPENING",
		3: "gripperState_GRABBING",
		4: "gripperState_IDLE",
		5: "gripperState_ANTI_SLIP_FORCE_CONTROLLING",
		6: "gripperState_EXITING",
	}
)
