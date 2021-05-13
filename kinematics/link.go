package kinematics

import (
	"gonum.org/v1/gonum/graph"
)

// Element TODO
type Element interface {
	ForwardPosition()
	ForwardVelocity()
}

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
}
