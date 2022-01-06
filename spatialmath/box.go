package spatialmath

import (
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/utils"
)

// BoxCreator implements the VolumeCreator interface for box structs.
type boxCreator struct {
	halfSize r3.Vector
	offset   Pose
}

// box is a collision geometry that represents a 3D rectangular prism, it has a pose and half size that fully define it.
type box struct {
	pose     Pose
	halfSize r3.Vector
}

// NewBox instantiates a BoxCreator class, which allows instantiating boxes given only a pose.
// These boxes have dimensions given by the provided halfSize vector.
func NewBox(halfSize r3.Vector) VolumeCreator {
	return &boxCreator{halfSize, NewZeroPose()}
}

// NewBoxFromOffset instantiates a BoxCreator class, which allows instantiating boxes given only a pose which is applied
// at the specified offset from the pose. These boxes have dimensions given by the provided halfSize vector.
func NewBoxFromOffset(halfSize r3.Vector, offset Pose) VolumeCreator {
	return &boxCreator{halfSize, offset}
}

// NewVolume instantiates a new box from a BoxCreator class.
func (bc *boxCreator) NewVolume(pose Pose) Volume {
	b := &box{bc.offset, bc.halfSize}
	b.Transform(pose)
	return b
}

// Pose returns the pose of the box.
func (b *box) Pose() Pose {
	return b.pose
}

// AlmostEqual compares the box with another volume and checks if they are equivalent.
func (b *box) AlmostEqual(v Volume) bool {
	other, ok := v.(*box)
	if !ok {
		return false
	}
	return PoseAlmostEqual(b.pose, other.pose) && utils.R3VectorAlmostEqual(b.halfSize, other.halfSize, 1e-8)
}

// Transform premultiplies the box pose with a transform, allowing the box to be moved in space.
func (b *box) Transform(toPremultiply Pose) {
	b.pose = Compose(toPremultiply, b.pose)
}

// CollidesWith checks if the given box collides with the given volume and returns true if it does.
func (b *box) CollidesWith(v Volume) (bool, error) {
	if other, ok := v.(*box); ok {
		return boxVsBoxCollision(b, other), nil
	}
	return true, errors.Errorf("collisions between box and %T are not supported", v)
}

// CollidesWith checks if the given box collides with the given volume and returns true if it does.
func (b *box) DistanceFrom(v Volume) (float64, error) {
	if other, ok := v.(*box); ok {
		return boxVsBoxDistance(b, other), nil
	}
	return math.Inf(-1), errors.Errorf("collisions between box and %T are not supported", v)
}

// boxVsBoxCollision takes two boxes as arguments and returns a bool describing if they are in collision,
// true == collision / false == no collision.
func boxVsBoxCollision(a, b *box) bool {
	positionDelta := PoseDelta(a.pose, b.pose).Point()
	rmA := a.pose.Orientation().RotationMatrix()
	rmB := b.pose.Orientation().RotationMatrix()
	for i := 0; i < 3; i++ {
		if separatingAxisTest(positionDelta, rmA.Row(i), a, b) > 0 {
			return false
		}
		if separatingAxisTest(positionDelta, rmB.Row(i), a, b) > 0 {
			return false
		}
		for j := 0; j < 3; j++ {
			if separatingAxisTest(positionDelta, rmA.Row(i).Cross(rmB.Row(j)), a, b) > 0 {
				return false
			}
		}
	}
	return true
}

// boxVsBoxDistance takes two boxes as arguments and returns a floating point number.  If this number is nonpositive it represents
// the penetration depth for the two boxes, which are in collision.  If the returned float is positive it represents
// a lower bound on the separation distance for the two boxes, which are not in collision.
// NOTES: calculating the true separation distance is a computationally infeasible problem
//		  the "minimum translation vector" (MTV) can also be computed here but is not currently as there is no use for it yet
// references:  https://comp.graphics.algorithms.narkive.com/jRAgjIUh/obb-obb-distance-calculation
//				https://dyn4j.org/2010/01/sat/#sat-nointer
func boxVsBoxDistance(a, b *box) float64 {
	positionDelta := PoseDelta(a.pose, b.pose).Point()
	rmA := a.pose.Orientation().RotationMatrix()
	rmB := b.pose.Orientation().RotationMatrix()
	dists := make([]float64, 15)
	for i := 0; i < 3; i++ {
		dists[5*i] = separatingAxisTest(positionDelta, rmA.Row(i), a, b)
		dists[5*i+1] = separatingAxisTest(positionDelta, rmB.Row(i), a, b)
		for j := 0; j < 3; j++ {
			edgeCP := rmA.Row(i).Cross(rmB.Row(j))
			// if edges are parallel, this check is already accounted for by a face pair - ignore case
			if edgeCP.Norm() == 0 {
				dists[5*i+j+2] = math.Inf(-1)
			} else {
				dists[5*i+j+2] = separatingAxisTest(positionDelta, edgeCP, a, b)
			}
		}
	}

	// returned distance in either case will be max of separations along axes
	max := dists[0]
	for i := 1; i < 15; i++ {
		if dists[i] > max {
			max = dists[i]
		}
	}
	return max
}

// separatingAxisTest projects two boxes onto the given plane and compute how much distance is between them along
// this plane.  Per the separating hyperplane theorem, if such a plane exists (and a positive number is returned)
// this proves that there is no collision between the boxes
// references:  https://gamedev.stackexchange.com/questions/112883/simple-3d-obb-collision-directx9-c
//              https://gamedev.stackexchange.com/questions/25397/obb-vs-obb-collision-detection
//			    https://www.cs.bgu.ac.il/~vgp192/wiki.files/Separating%20Axis%20Theorem%20for%20Oriented%20Bounding%20Boxes.pdf
//			    https://gamedev.stackexchange.com/questions/112883/simple-3d-obb-collision-directx9-c
func separatingAxisTest(positionDelta, plane r3.Vector, a, b *box) float64 {
	rmA := a.pose.Orientation().RotationMatrix()
	rmB := b.pose.Orientation().RotationMatrix()
	return math.Abs(positionDelta.Dot(plane)) - (math.Abs(rmA.Row(0).Mul(a.halfSize.X).Dot(plane)) +
		math.Abs(rmA.Row(1).Mul(a.halfSize.Y).Dot(plane)) +
		math.Abs(rmA.Row(2).Mul(a.halfSize.Z).Dot(plane)) +
		math.Abs(rmB.Row(0).Mul(b.halfSize.X).Dot(plane)) +
		math.Abs(rmB.Row(1).Mul(b.halfSize.Y).Dot(plane)) +
		math.Abs(rmB.Row(2).Mul(b.halfSize.Z).Dot(plane)))
}
