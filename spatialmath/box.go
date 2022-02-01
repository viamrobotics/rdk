package spatialmath

import (
	"encoding/json"
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
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

// NewBox instantiates a BoxCreator class, which allows instantiating boxes given only a pose which is applied
// at the specified offset from the pose. These boxes have dimensions given by the provided halfSize vector.
func NewBox(dims r3.Vector, offset Pose) (VolumeCreator, error) {
	if dims.X == 0 || dims.Y == 0 || dims.Z == 0 {
		return nil, errors.New("box dimensions can not be zero")
	}
	return &boxCreator{dims.Mul(0.5), offset}, nil
}

// NewVolume instantiates a new box from a BoxCreator class.
func (bc *boxCreator) NewVolume(pose Pose) Volume {
	b := &box{bc.offset, bc.halfSize}
	b.Transform(pose)
	return b
}

func (bc *boxCreator) MarshalJSON() ([]byte, error) {
	return json.Marshal(VolumeConfig{
		Type: "box",
		X:    2 * bc.halfSize.X,
		Y:    2 * bc.halfSize.Y,
		Z:    2 * bc.halfSize.Z,
	})
}

// Pose returns the pose of the box.
func (b *box) Pose() Pose {
	return b.pose
}

func (b *box) Vertices() []r3.Vector {
	vertices := make([]r3.Vector, 8)
	for i, x := range []float64{1, -1} {
		for j, y := range []float64{1, -1} {
			for k, z := range []float64{1, -1} {
				offset := NewPoseFromPoint(r3.Vector{X: x * b.halfSize.X, Y: y * b.halfSize.Y, Z: z * b.halfSize.Z})
				vertices[4*i+2*j+k] = Compose(b.pose, offset).Point()
			}
		}
	}
	return vertices
}

// AlmostEqual compares the box with another volume and checks if they are equivalent.
func (b *box) AlmostEqual(v Volume) bool {
	other, ok := v.(*box)
	if !ok {
		return false
	}
	return PoseAlmostEqual(b.pose, other.pose) && R3VectorAlmostEqual(b.halfSize, other.halfSize, 1e-8)
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
//        the "minimum translation vector" (MTV) can also be computed here but is not currently as there is no use for it yet
// references:  https://comp.graphics.algorithms.narkive.com/jRAgjIUh/obb-obb-distance-calculation
//              https://dyn4j.org/2010/01/sat/#sat-nointer
func boxVsBoxDistance(a, b *box) float64 {
	positionDelta := PoseDelta(a.pose, b.pose).Point()
	rmA := a.pose.Orientation().RotationMatrix()
	rmB := b.pose.Orientation().RotationMatrix()

	// iterate over axes of box
	max := math.Inf(-1)
	for i := 0; i < 3; i++ {
		// project onto face of box A
		separation := separatingAxisTest(positionDelta, rmA.Row(i), a, b)
		if separation > max {
			max = separation
		}

		// project onto face of box B
		separation = separatingAxisTest(positionDelta, rmB.Row(i), a, b)
		if separation > max {
			max = separation
		}

		// project onto a plane created by cross product of two edges from boxes
		for j := 0; j < 3; j++ {
			crossProductPlane := rmA.Row(i).Cross(rmB.Row(j))

			// if edges are parallel, this check is already accounted for by one of the face projections, so skip this case
			if crossProductPlane.Norm() != 0 {
				separation = separatingAxisTest(positionDelta, crossProductPlane, a, b)
				if separation > max {
					max = separation
				}
			}
		}
	}
	return max
}

// separatingAxisTest projects two boxes onto the given plane and compute how much distance is between them along
// this plane.  Per the separating hyperplane theorem, if such a plane exists (and a positive number is returned)
// this proves that there is no collision between the boxes
// references:  https://gamedev.stackexchange.com/questions/112883/simple-3d-obb-collision-directx9-c
//              https://gamedev.stackexchange.com/questions/25397/obb-vs-obb-collision-detection
//              https://www.cs.bgu.ac.il/~vgp192/wiki.files/Separating%20Axis%20Theorem%20for%20Oriented%20Bounding%20Boxes.pdf
//              https://gamedev.stackexchange.com/questions/112883/simple-3d-obb-collision-directx9-c
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
