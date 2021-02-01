package arm

import (
	"fmt"
	"github.com/echolabsinc/robotcore/utils"
)

type Position struct {
	X, Y, Z float64 // meters of the end effector from the base

	Rx, Ry, Rz float64 // rotations around each axis, in degrees
}

func (p Position) NondelimitedString() string {
	return fmt.Sprintf("%f %f %f %f %f %f",
		p.X, p.Y, p.Z, p.Rx, p.Ry, p.Rz)
}

func (p Position) RxRadians() float64 {
	return utils.DegToRad(p.Rx)
}

func (p Position) RyRadians() float64 {
	return utils.DegToRad(p.Ry)
}

func (p Position) RzRadians() float64 {
	return utils.DegToRad(p.Rz)
}

func NewPositionFromMetersAndRadians(x, y, z, rx, ry, rz float64) Position {
	return Position{
		X:  x,
		Y:  y,
		Z:  z,
		Rx: utils.RadToDeg(rx),
		Ry: utils.RadToDeg(ry),
		Rz: utils.RadToDeg(rz),
	}
}

// this is probably wrong, just trying to start abstracting
type Arm interface {
	CurrentPosition() (Position, error)
	MoveToPosition(c Position) error

	MoveToJointPositions([]float64) error           // TODO(erh): make it clear the units
	CurrentJointPositions() ([]float64, error)      // TODO(erh): make it clear the units
	JointMoveDelta(joint int, amount float64) error // TODO(erh): make it clear the units

	Close()
}
