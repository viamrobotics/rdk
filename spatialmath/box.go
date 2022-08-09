package spatialmath

import (
	"encoding/json"
	"math"

	"github.com/golang/geo/r3"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/utils"
)

// BoxCreator implements the GeometryCreator interface for box structs.
type boxCreator struct {
	halfSize r3.Vector
	offset   Pose
}

// box is a collision geometry that represents a 3D rectangular prism, it has a pose and half size that fully define it.
type box struct {
	pose     Pose
	halfSize [3]float64
}

// NewBoxCreator instantiates a BoxCreator class, which allows instantiating boxes given only a pose which is applied
// at the specified offset from the pose. These boxes have dimensions given by the provided halfSize vector.
func NewBoxCreator(dims r3.Vector, offset Pose) (GeometryCreator, error) {
	if dims.X <= 0 || dims.Y <= 0 || dims.Z <= 0 {
		return nil, newBadGeometryDimensionsError(&box{})
	}
	return &boxCreator{dims.Mul(0.5), offset}, nil
}

// NewGeometry instantiates a new box from a BoxCreator class.
func (bc *boxCreator) NewGeometry(pose Pose) Geometry {
	return &box{Compose(bc.offset, pose), [3]float64{bc.halfSize.X, bc.halfSize.Y, bc.halfSize.Z}}
}

func (bc *boxCreator) MarshalJSON() ([]byte, error) {
	config, err := NewGeometryConfig(bc.offset)
	if err != nil {
		return nil, err
	}
	config.Type = "box"
	config.X = 2 * bc.halfSize.X
	config.Y = 2 * bc.halfSize.Y
	config.Z = 2 * bc.halfSize.Z
	return json.Marshal(config)
}

// NewBox instantiates a new box Geometry.
func NewBox(pose Pose, dims r3.Vector) (Geometry, error) {
	if dims.X < 0 || dims.Y < 0 || dims.Z < 0 {
		return nil, newBadGeometryDimensionsError(&box{})
	}
	return &box{pose, [3]float64{0.5 * dims.X, 0.5 * dims.Y, 0.5 * dims.Z}}, nil
}

// Pose returns the pose of the box.
func (b *box) Pose() Pose {
	return b.pose
}

// Vertices returns the vertices defining the box.
func (b *box) Vertices() []r3.Vector {
	vertices := make([]r3.Vector, 8)
	for i, x := range []float64{1, -1} {
		for j, y := range []float64{1, -1} {
			for k, z := range []float64{1, -1} {
				offset := NewPoseFromPoint(r3.Vector{X: x * b.halfSize[0], Y: y * b.halfSize[1], Z: z * b.halfSize[2]})
				vertices[4*i+2*j+k] = Compose(b.pose, offset).Point()
			}
		}
	}
	return vertices
}

// AlmostEqual compares the box with another geometry and checks if they are equivalent.
func (b *box) AlmostEqual(g Geometry) bool {
	other, ok := g.(*box)
	if !ok {
		return false
	}
	for i := 0; i < 3; i++ {
		if !utils.Float64AlmostEqual(b.halfSize[i], other.halfSize[i], 1e-8) {
			return false
		}
	}
	return PoseAlmostEqual(b.pose, other.pose)
}

// Transform premultiplies the box pose with a transform, allowing the box to be moved in space.
func (b *box) Transform(toPremultiply Pose) Geometry {
	return &box{Compose(toPremultiply, b.pose), b.halfSize}
}

// ToProto converts the box to a Geometry proto message.
func (b *box) ToProtobuf() *commonpb.Geometry {
	return &commonpb.Geometry{
		Center: PoseToProtobuf(b.pose),
		GeometryType: &commonpb.Geometry_Box{
			Box: &commonpb.RectangularPrism{DimsMm: &commonpb.Vector3{
				X: 2 * b.halfSize[0],
				Y: 2 * b.halfSize[1],
				Z: 2 * b.halfSize[2],
			}},
		},
	}
}

// CollidesWith checks if the given box collides with the given geometry and returns true if it does.
func (b *box) CollidesWith(g Geometry) (bool, error) {
	if other, ok := g.(*box); ok {
		return boxVsBoxCollision(b, other), nil
	}
	if other, ok := g.(*sphere); ok {
		return sphereVsBoxCollision(other, b), nil
	}
	if other, ok := g.(*point); ok {
		return pointVsBoxCollision(b, other.pose.Point()), nil
	}
	return true, newCollisionTypeUnsupportedError(b, g)
}

// CollidesWith checks if the given box collides with the given geometry and returns true if it does.
func (b *box) DistanceFrom(g Geometry) (float64, error) {
	if other, ok := g.(*box); ok {
		return boxVsBoxDistance(b, other), nil
	}
	if other, ok := g.(*sphere); ok {
		return sphereVsBoxDistance(other, b), nil
	}
	if other, ok := g.(*point); ok {
		return pointVsBoxDistance(b, other.pose.Point()), nil
	}
	return math.Inf(-1), newCollisionTypeUnsupportedError(b, g)
}

func (b *box) EncompassedBy(g Geometry) (bool, error) {
	if other, ok := g.(*box); ok {
		return boxInBox(b, other), nil
	}
	if other, ok := g.(*sphere); ok {
		return boxInSphere(b, other), nil
	}
	if _, ok := g.(*point); ok {
		return false, nil
	}
	return false, newCollisionTypeUnsupportedError(b, g)
}

// closestPoint returns the closest point on the specified box to the specified point
// Reference: https://github.com/gszauer/GamePhysicsCookbook/blob/a0b8ee0c39fed6d4b90bb6d2195004dfcf5a1115/Code/Geometry3D.cpp#L165
func (b *box) closestPoint(pt r3.Vector) r3.Vector {
	result := b.pose.Point()
	direction := pt.Sub(result)
	rm := b.pose.Orientation().RotationMatrix()
	for i := 0; i < 3; i++ {
		axis := rm.Row(i)
		distance := direction.Dot(axis)
		if distance > b.halfSize[i] {
			distance = b.halfSize[i]
		} else if distance < -b.halfSize[i] {
			distance = -b.halfSize[i]
		}
		result = result.Add(axis.Mul(distance))
	}
	return result
}

// penetrationDepth returns the minimum distance needed to move a pt inside the box to the edge of the box.
func (b *box) penetrationDepth(pt r3.Vector) float64 {
	direction := pt.Sub(b.pose.Point())
	rm := b.pose.Orientation().RotationMatrix()
	min := math.Inf(1)
	for i := 0; i < 3; i++ {
		axis := rm.Row(i)
		projection := direction.Dot(axis)
		if distance := math.Abs(projection - b.halfSize[i]); distance < min {
			min = distance
		}
		if distance := math.Abs(projection + b.halfSize[i]); distance < min {
			min = distance
		}
	}
	return min
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

// boxInBox returns a bool describing if the inner box is completely encompassed by the outer box.
func boxInBox(inner, outer *box) bool {
	for _, vertex := range inner.Vertices() {
		if !pointVsBoxCollision(outer, vertex) {
			return false
		}
	}
	return true
}

// boxInSphere returns a bool describing if the given box is completely encompassed by the given sphere.
func boxInSphere(b *box, s *sphere) bool {
	for _, vertex := range b.Vertices() {
		if sphereVsPointDistance(s, vertex) > 0 {
			return false
		}
	}
	return sphereVsPointDistance(s, b.pose.Point()) <= 0
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
	sum := math.Abs(positionDelta.Dot(plane))
	for i := 0; i < 3; i++ {
		sum -= math.Abs(rmA.Row(i).Mul(a.halfSize[i]).Dot(plane))
		sum -= math.Abs(rmB.Row(i).Mul(b.halfSize[i]).Dot(plane))
	}
	return sum
}
