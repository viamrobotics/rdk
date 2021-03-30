package api

import (
	"go.viam.com/robotcore/utils"
)

type ArmPosition struct {
	X, Y, Z float64 // meters of the end effector from the base

	Rx, Ry, Rz float64 // angular orientation about each axis, in degrees
}

func (p ArmPosition) RxRadians() float64 {
	return utils.DegToRad(p.Rx)
}

func (p ArmPosition) RyRadians() float64 {
	return utils.DegToRad(p.Ry)
}

func (p ArmPosition) RzRadians() float64 {
	return utils.DegToRad(p.Rz)
}

func NewPositionFromMetersAndRadians(x, y, z, rx, ry, rz float64) ArmPosition {
	return ArmPosition{
		X:  x,
		Y:  y,
		Z:  z,
		Rx: utils.RadToDeg(rx),
		Ry: utils.RadToDeg(ry),
		Rz: utils.RadToDeg(rz),
	}
}

type JointPositions struct {
	Degrees []float64 `json:"degreesList"`
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
	CurrentPosition() (ArmPosition, error)
	MoveToPosition(c ArmPosition) error

	MoveToJointPositions(JointPositions) error
	CurrentJointPositions() (JointPositions, error)

	JointMoveDelta(joint int, amount float64) error // TODO(erh): make it clear the units

	Close()
}
