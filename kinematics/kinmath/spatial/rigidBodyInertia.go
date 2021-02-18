package spatial

import (
	"github.com/go-gl/mathgl/mgl64"
)

type RigidBodyInertia struct {
	Inertia mgl64.Vec3
	Cog     mgl64.Vec3
	Mass    mgl64.Vec3
}

//~ func (r *RigidBodyInertia) Times (other *MotionVector) ForceVector {
//~ res := ForceVector{}
//~ res.Moment = r.Inertia * other.Angular + r.Cog.cross(other.Linear)
//~ res.Force = r.Mass * other.Linear - r.Cog.cross(other.Angular)

//~ return res
//~ }
