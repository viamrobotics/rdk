package kinematics

import (
	//~ "fmt"
	//~ "reflect"
	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

// Generally speaking, a Joint will attach a Body to a Frame
// And a Fixed will attach a Frame to a Body
// Exceptions are the head of the tree where we are just starting the robot from World
type Model struct {
	manufacturer string
	name         string
	tree         *simple.DirectedGraph
	root         int64
	nextID       uint64
	Nodes        map[int64]*Frame
	Edges        map[graph.Edge]Link
	Frames       []*Frame
	Bodies       []*Frame
	Joints       []*Joint
	// Links are Fixeds and Joints
	Links []Link
	// Elements are Links and Frames and probably not needed
	Elements []Element
	// Leaves are the node IDs of end effectors
	Leaves           []int64
	GammaPosition    *mgl64.MatMxN
	InvGammaPosition *mgl64.MatMxN
	Home             []float64
	Jacobian         *mgl64.MatMxN
	InvJacobian      *mgl64.MatMxN
}

// Constructor for a model
func NewModel() *Model {
	m := Model{}
	m.tree = simple.NewDirectedGraph()
	m.Nodes = make(map[int64]*Frame)
	m.Edges = make(map[graph.Edge]Link)
	return &m
}

// NextID will return the next ID to use for a node in the directed graph
// Hypothetically this could eventually overflow
// If we ever run one model long enough to need 20 sextillion tree nodes, we will deal with it at that time
func (m *Model) NextID() int64 {
	id := int64(m.nextID)
	m.nextID++
	return id
}

func (m *Model) Add(frame *Frame) {
	frame.SetVertexDescriptor(m.NextID())
	m.tree.AddNode(simple.Node(frame.GetVertexDescriptor()))
	if frame.IsWorld {
		m.root = frame.GetVertexDescriptor()
	}
	m.Nodes[frame.GetVertexDescriptor()] = frame
}

// Annoyingly, pointers aren't implemented on edges with simple.DirectedGraph
func (m *Model) AddEdge(frameA, frameB *Frame) graph.Edge {
	edge := m.tree.NewEdge(m.tree.Node(frameA.GetVertexDescriptor()), m.tree.Node(frameB.GetVertexDescriptor()))
	m.tree.SetEdge(edge)
	return edge
}

//~ func (m *Model) Leaves() []graph.Node{

//~ }
//~ func (m *Model) getOperationalDof() int{
//~ return len(m.Leaves())
//~ }

func (m *Model) GetJoint(i int) *Joint {
	return m.Joints[i]
}

func (m *Model) GetJoints() int {
	return len(m.Joints)
}

// GetOperationalDof returns the number of end effectors
func (m *Model) GetOperationalDof() int {
	return len(m.Leaves)
}

// GetDof returns the sum of Dof from all joints
func (m *Model) GetDof() int {
	dof := 0
	for _, joint := range m.Joints {
		dof += joint.GetDof()
	}
	return dof
}

// GetDofPosition returns the sum of DofPosition from all joints
func (m *Model) GetDofPosition() int {
	dof := 0
	for _, joint := range m.Joints {
		dof += joint.GetDofPosition()
	}
	return dof
}

// SetPosition sets joint angles to specific locations in ***radians***
func (m *Model) SetPosition(newPos []float64) {
	newPosVec := mgl64.NewVecNFromData(newPos)
	newPosVec = m.GammaPosition.MulNx1(newPosVec, newPosVec)
	idx := 0
	for j := 0; j < len(m.Joints); j++ {
		m.Joints[j].SetPosition(newPosVec.Raw()[idx : idx+m.Joints[j].GetDofPosition()])
		idx += m.Joints[j].GetDofPosition()
	}
}

// GetPosition returns the array of GetPosition from all joints
func (m *Model) GetPosition() []float64 {
	var jointPos []float64
	for _, joint := range m.Joints {
		jointPos = append(jointPos, joint.GetPosition()...)
	}
	return jointPos
}

// SetVelocity sets joint velocities
func (m *Model) SetVelocity(newVel []float64) {
	newPosVec := mgl64.NewVecNFromData(newVel)
	newPosVec = m.GammaPosition.MulNx1(newPosVec, newPosVec)
	idx := 0
	for j := 0; j < len(m.Joints); j++ {
		m.Joints[j].SetVelocity(newPosVec.Raw()[idx : idx+m.Joints[j].GetDof()])
		idx += m.Joints[j].GetDof()
	}
}

func (m *Model) Normalize(pos []float64) []float64 {
	i := 0
	var normalized []float64
	for _, joint := range m.Joints {
		normalized = append(normalized, joint.Normalize(pos[i:i+joint.GetDofPosition()])...)
		i += joint.GetDofPosition()
	}
	return normalized
}

func (m *Model) IsValid(pos []float64) bool {
	i := 0
	for _, joint := range m.Joints {
		if !(joint.IsValid(pos[i : i+joint.GetDofPosition()])) {
			return false
		}
		i += joint.GetDofPosition()
	}
	return true
}

func (m *Model) Update() {
	m.Frames = nil
	m.Bodies = nil
	m.Joints = nil
	m.Links = nil

	m.UpdateOnFrame(m.root)
}

func (m *Model) UpdateOnFrame(frameID int64) {
	frame := m.Nodes[frameID]
	m.Frames = append(m.Frames, frame)
	m.Elements = append(m.Elements, frame)
	if frame.IsBody {
		m.Bodies = append(m.Bodies, frame)
	}
	outNodes := m.tree.From(frameID)
	hadNode := false
	// Iterate over all nodes connecting to the root
	for outNodes.Next() {
		hadNode = true
		nodeID := outNodes.Node()
		outFrame := m.Nodes[nodeID.ID()]
		edge := m.tree.Edge(frameID, nodeID.ID())
		link := m.Edges[edge]
		m.Links = append(m.Links, link)
		m.Elements = append(m.Elements, link)

		link.SetIn(frame)
		link.SetOut(outFrame)

		if joint, ok := link.(*Joint); ok {
			m.Joints = append(m.Joints, joint)
		}
		// Recursively go down each branch of the tree to its end
		m.UpdateOnFrame(nodeID.ID())
	}

	if !hadNode {
		m.Leaves = append(m.Leaves, frameID)
	}

	// TODO: Determine whether this needs to be run on every single call- should be able to eschew in recursion
	// TODO: GammaVelocity may need to be added
	m.GammaPosition = mgl64.NewMatrix(m.GetDofPosition(), m.GetDofPosition())
	mgl64.IdentN(m.GammaPosition, m.GetDofPosition())

	m.InvGammaPosition = mgl64.NewMatrix(m.GetDofPosition(), m.GetDofPosition())
	mgl64.IdentN(m.InvGammaPosition, m.GetDofPosition())

	m.Home = make([]float64, m.GetDofPosition())
}
