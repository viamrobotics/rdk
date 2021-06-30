package kinematics

import (
	"go.viam.com/core/spatialmath"

	"gonum.org/v1/gonum/graph"
)

// Link TODO
type Link interface {
	ForwardPosition()
	ForwardVelocity()
	SetEdgeDescriptor(graph.Edge)
	GetEdgeDescriptor() graph.Edge
	SetName(string)
	GetName() string
	SetIn(*Frame)
	GetIn() *Frame
	SetOut(*Frame)
	GetOut() *Frame
	Quaternion() *spatialmath.DualQuaternion
}
