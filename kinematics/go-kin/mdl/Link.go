package mdl

import(
	"gonum.org/v1/gonum/graph"
)

type Element interface{
	ForwardPosition()
	ForwardVelocity()
}

type Link interface{
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
