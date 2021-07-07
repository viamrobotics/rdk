package kinematics

import (
	"go.viam.com/core/spatialmath"
)

// A Transform can 
type Transform interface {
	Parent() string
	Quaternion() *spatialmath.DualQuaternion
}

// Link TODO
type Link struct {
	quat      *spatialmath.DualQuaternion
	parent    string
}

// NewTransform TODO
func NewLink(parent string) *Link {
	l := Link{quat: spatialmath.NewDualQuaternion(), parent: parent}
	return &l
}

// Quaternion TODO
func (l *Link) Quaternion() *spatialmath.DualQuaternion {
	return l.quat
}

func (l *Link) Parent() string {
	return l.parent
}
