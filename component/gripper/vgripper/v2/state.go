package vgripper

// GripperState describes the action the gripper is performing.
type GripperState int32

const (
	gripperStateUNSPECIFIED              GripperState = 0
	gripperStateCALIBRATING              GripperState = 1
	gripperStateOPENING                  GripperState = 2
	gripperStateGRABBING                 GripperState = 3
	gripperStateIDLE                     GripperState = 4
	gripperStateANTISLIPFORCECONTROLLING GripperState = 5
)

// Enum value maps for DirectionRelative.
// TODO: write tests for this to make sure that all conversions are a closed loop;
// to make sure human error doesn't happen here
var (
	gripperStateName = map[GripperState]string{
		0: "gripperStateUNSPECIFIED",
		1: "gripperStateCALIBRATING",
		2: "gripperStateOPENING",
		3: "gripperStateGRABBING",
		4: "gripperStateIDLE",
		5: "gripperStateANTISLIPFORCECONTROLLING",
	}
)
