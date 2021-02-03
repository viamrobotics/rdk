package arm

import (
	"fmt"

	"github.com/viamrobotics/robotcore/utils"
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

type JointPositions struct {
	Degrees []float64
}

func (jp JointPositions) Radians() []float64 {
	n := make([]float64, len(jp.Degrees))
	for idx, d := range jp.Degrees {
		n[idx] = utils.DegToRad(d)
	}
	return n
}

func JointPositionsFromRadians(radians []float64) JointPositions {
	n := make([]float64, len(radians))
	for idx, a := range radians {
		n[idx] = utils.RadToDeg(a)
	}
	return JointPositions{n}
}

// -----

type Arm interface {
	CurrentPosition() (Position, error)
	MoveToPosition(c Position) error

	MoveToJointPositions(JointPositions) error
	CurrentJointPositions() (JointPositions, error)

	JointMoveDelta(joint int, amount float64) error // TODO(erh): make it clear the units

	Close()
}
