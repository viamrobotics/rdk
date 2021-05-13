package kinematics

import (
	"math/rand"

	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

// XYZWeights TODO
type XYZWeights struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// DistanceConfig values are used to augment the distance check for a given IK solution.
// For each component of a 6d pose, the distance from current position to goal is
// squared and then multiplied by the corresponding weight in this struct. The results
// are summed and that sum must be below a certain threshold.
// So values > 1 forces the IK algorithm to get that value closer to perfect than it
// otherwise would have, and values < 1 cause it to be more lax. A value of 0.0 will cause
// that dimension to not be considered at all.
type DistanceConfig struct {
	Trans  XYZWeights `json:"translation"`
	Orient XYZWeights `json:"orientation"`
}

// Model TODO
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
	RandSeed         *rand.Rand
	DistCfg          DistanceConfig
}

// NewModel constructs a new model.
func NewModel() *Model {
	m := Model{}
	m.tree = simple.NewDirectedGraph()
	m.Nodes = make(map[int64]*Frame)
	m.Edges = make(map[graph.Edge]Link)
	m.RandSeed = rand.New(rand.NewSource(1))
	m.DistCfg = DistanceConfig{XYZWeights{1.0, 1.0, 1.0}, XYZWeights{1.0, 1.0, 1.0}}
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

// SetSeed TODO
func (m *Model) SetSeed(seed int64) {
	m.RandSeed = rand.New(rand.NewSource(seed))
}

// Add TODO
func (m *Model) Add(frame *Frame) {
	frame.SetVertexDescriptor(m.NextID())
	m.tree.AddNode(simple.Node(frame.GetVertexDescriptor()))
	if frame.IsWorld {
		m.root = frame.GetVertexDescriptor()
	}
	m.Nodes[frame.GetVertexDescriptor()] = frame
}

// AddEdge TODO
// Annoyingly, pointers aren't implemented on edges with simple.DirectedGraph
func (m *Model) AddEdge(frameA, frameB *Frame) graph.Edge {
	edge := m.tree.NewEdge(m.tree.Node(frameA.GetVertexDescriptor()), m.tree.Node(frameB.GetVertexDescriptor()))
	m.tree.SetEdge(edge)
	return edge
}

// RandomJointPositions generates a list of radian joint positions that are random but valid for each joint.
func (m *Model) RandomJointPositions() []float64 {
	var jointPos []float64
	for _, joint := range m.Joints {
		jointPos = append(jointPos, joint.RandomJointPositions(m.RandSeed)...)
	}
	return jointPos
}

// GetJoint TODO.
func (m *Model) GetJoint(i int) *Joint {
	return m.Joints[i]
}

// GetJoints TODO.
func (m *Model) GetJoints() int {
	return len(m.Joints)
}

// GetOperationalDof returns the number of end effectors.
func (m *Model) GetOperationalDof() int {
	return len(m.Leaves)
}

// GetDof returns the sum of Dof from all joints- Should sum to the total degrees of freedom for the robot
// In other words, if the robot consists of a 6dof arm and an additional 4dof arm, this will return 10.
func (m *Model) GetDof() int {
	dof := 0
	for _, joint := range m.Joints {
		dof += joint.GetDof()
	}
	return dof
}

// GetDofPosition returns the sum of DofPosition from all joints. Equal to GetDof() for standard 1dof revolute joints.
func (m *Model) GetDofPosition() int {
	dof := 0
	for _, joint := range m.Joints {
		dof += joint.GetDofPosition()
	}
	return dof
}

// SetPosition sets joint angles to specific locations in radians.
func (m *Model) SetPosition(newPos []float64) {
	newPosVec := mgl64.NewVecNFromData(newPos)
	newPosVec = m.GammaPosition.MulNx1(newPosVec, newPosVec)
	idx := 0
	for j := 0; j < len(m.Joints); j++ {
		m.Joints[j].SetPosition(newPosVec.Raw()[idx : idx+m.Joints[j].GetDofPosition()])
		idx += m.Joints[j].GetDofPosition()
	}
}

// GetPosition returns the array of GetPosition from all joints.
func (m *Model) GetPosition() []float64 {
	var jointPos []float64
	for _, joint := range m.Joints {
		jointPos = append(jointPos, joint.GetPosition()...)
	}
	return jointPos
}

// GetMinimum returns the array of GetPosition from all joints.
func (m *Model) GetMinimum() []float64 {
	var jointMin []float64
	for _, joint := range m.Joints {
		jointMin = append(jointMin, joint.GetMinimum()...)
	}
	return jointMin
}

// GetMaximum returns the array of GetPosition from all joints.
func (m *Model) GetMaximum() []float64 {
	var jointMax []float64
	for _, joint := range m.Joints {
		jointMax = append(jointMax, joint.GetMaximum()...)
	}
	return jointMax
}

// SetVelocity sets joint velocities.
func (m *Model) SetVelocity(newVel []float64) {
	newPosVec := mgl64.NewVecNFromData(newVel)
	newPosVec = m.GammaPosition.MulNx1(newPosVec, newPosVec)
	idx := 0
	for j := 0; j < len(m.Joints); j++ {
		m.Joints[j].SetVelocity(newPosVec.Raw()[idx : idx+m.Joints[j].GetDof()])
		idx += m.Joints[j].GetDof()
	}
}

// Normalize TODO.
func (m *Model) Normalize(pos []float64) []float64 {
	i := 0
	var normalized []float64
	for _, joint := range m.Joints {
		normalized = append(normalized, joint.Normalize(pos[i:i+joint.GetDofPosition()])...)
		i += joint.GetDofPosition()
	}
	return normalized
}

// IsValid TODO.
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

// Update TODO.
func (m *Model) Update() {
	m.Frames = nil
	m.Bodies = nil
	m.Joints = nil
	m.Links = nil

	m.UpdateOnFrame(m.root)
}

// UpdateOnFrame TODO.
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

	// TODO(pl): Determine whether this needs to be run on every single call- should be able to eschew in recursion
	// TODO(pl): GammaVelocity may need to be added
	m.GammaPosition = mgl64.NewMatrix(m.GetDofPosition(), m.GetDofPosition())
	mgl64.IdentN(m.GammaPosition, m.GetDofPosition())

	m.InvGammaPosition = mgl64.NewMatrix(m.GetDofPosition(), m.GetDofPosition())
	mgl64.IdentN(m.InvGammaPosition, m.GetDofPosition())

	m.Home = make([]float64, m.GetDofPosition())
}
