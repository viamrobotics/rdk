package kinematics

import (
	"go.viam.com/core/spatialmath"
)

// A Transform can be a link or a joint.
type Transform interface {
	Parent() string
	Quaternion() *spatialmath.DualQuaternion
}

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

// Quaternion returns the quaternion associated with the link
func (l *Link) Quaternion() *spatialmath.DualQuaternion {
	return l.quat
}

// Parent will return the name of the next transform up the kinematics chain from this link.
func (l *Link) Parent() string {
	return l.parent
}
