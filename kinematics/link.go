package kinematics

import (
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/spatialmath"
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

// ParentName will return the name of the next transform up the kinematics chain from this link.
func (l *Link) ParentName() string {
	return l.parent
}

// Dof is zero for a link
func (l *Link) Dof() int {
	return 0
}

// Parent TODO to be implemented
func (l *Link) Parent() referenceframe.Frame {
	return nil
}

// Name TODO to be implemented
func (j *Link) Name() string {
	return ""
}
