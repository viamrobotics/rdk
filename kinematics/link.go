package kinematics

import (
	"go.viam.com/core/spatialmath"
	"go.viam.com/core/referenceframe"
)

// Link defines a fixed link
type Link struct {
	quat   *spatialmath.DualQuaternion
	parent string
}

// NewLink creates a new link with the specified parent
func NewLink(parent string) *Link {
	l := Link{quat: spatialmath.NewDualQuaternion(), parent: parent}
	return &l
}

// Transform returns the quaternion associated with the link
func (l *Link) Transform(input []referenceframe.Input) *spatialmath.DualQuaternion {
	return l.quat
}

// Parent will return the name of the next transform up the kinematics chain from this link.
func (l *Link) Parent() string {
	return l.parent
}

// Dof is zero for a link
func (l *Link) Dof() int {
	return 0
}
