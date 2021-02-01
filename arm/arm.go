package arm

// this is probably wrong, just trying to start abstracting
type Arm interface {
	CurrentPosition() (CartesianInfo, error)
	MoveToPositionC(c CartesianInfo) error
	MoveToPosition(x, y, z, rx, ry, rz float64) error // TODO(erh): make it clear the units

	MoveToJointPositions([]float64) error           // TODO(erh): make it clear the units
	CurrentJointPositions() ([]float64, error)      // TODO(erh): make it clear the units
	JointMoveDelta(joint int, amount float64) error // TODO(erh): make it clear the units

	Close()
}
