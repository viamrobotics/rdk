package vgripper

// gripperState describes the action the gripper is performing.
type gripperState int32

const (
	gripperStateUnspecified = gripperState(iota)
	gripperStateCalibrating
	gripperStateOpening
	gripperStateGrabbing
	gripperStateIdle
	gripperStateAntiSlipForceControlling
)

// String returns a string for the gripperState.
func (gs gripperState) String() string {
	switch gs {
	case gripperStateUnspecified:
		return "gripperStateUnspecified"
	case gripperStateCalibrating:
		return "gripperStateCalibrating"
	case gripperStateOpening:
		return "gripperStateOpening"
	case gripperStateGrabbing:
		return "gripperStateGrabbing"
	case gripperStateIdle:
		return "gripperStateIdle"
	case gripperStateAntiSlipForceControlling:
		return "gripperStateAntiSlipForceControlling"
	default:
		return "unknown"
	}
}
