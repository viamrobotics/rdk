package arm

// this is probably wrong, just trying to start abstracting
type Arm interface {
	CurrentPosition() (CartesianInfo, error)
	MoveToPositionC(c CartesianInfo) error
	MoveToPosition(x, y, z, rx, ry, rz float64) error
	JointMoveDelta(joint int, amount float64) error

	Close()
}
