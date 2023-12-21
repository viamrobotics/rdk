package spatialmath

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/utils"
)

// Ordered list of box vertices.
var boxVertices = [8]r3.Vector{
	{1, 1, 1},
	{1, 1, -1},
	{1, -1, 1},
	{1, -1, -1},
	{-1, 1, 1},
	{-1, 1, -1},
	{-1, -1, 1},
	{-1, -1, -1},
}

// The sets of indices of the box vertices that tile the box exterior.
var boxTriangles = [12][3]int{
	{0, 1, 3},
	{0, 2, 3},
	{0, 1, 5},
	{0, 4, 5},
	{0, 2, 6},
	{0, 4, 6},
	{7, 1, 3},
	{7, 2, 3},
	{7, 1, 5},
	{7, 4, 5},
	{7, 2, 6},
	{7, 4, 6},
}

// Ordered list of box face normals.
var boxNormals = [6]r3.Vector{
	{1, 0, 0},
	{0, 1, 0},
	{0, 0, 1},
	{-1, 0, 0},
	{0, -1, 0},
	{0, 0, -1},
}

// box is a collision geometry that represents a 3D rectangular prism, it has a pose and half size that fully define it.
type box struct {
	pose            Pose
	halfSize        [3]float64
	boundingSphereR float64
	label           string
	mesh            *mesh
	rotMatrix       *RotationMatrix
	once            sync.Once
}

// NewBox instantiates a new box Geometry.
func NewBox(pose Pose, dims r3.Vector, label string) (Geometry, error) {
	// Negative dimensions not allowed. Zero dimensions are allowed for bounding boxes, etc.
	if dims.X < 0 || dims.Y < 0 || dims.Z < 0 {
		return nil, newBadGeometryDimensionsError(&box{})
	}
	halfSize := dims.Mul(0.5)
	return &box{
		pose:            pose,
		halfSize:        [3]float64{halfSize.X, halfSize.Y, halfSize.Z},
		boundingSphereR: halfSize.Norm(),
		label:           label,
	}, nil
}

// String returns a human readable string that represents the box.
func (b *box) String() string {
	return fmt.Sprintf("Type: Box | Position: X:%.1f, Y:%.1f, Z:%.1f | Dims: X:%.0f, Y:%.0f, Z:%.0f",
		b.pose.Point().X, b.pose.Point().Y, b.pose.Point().Z, 2*b.halfSize[0], 2*b.halfSize[1], 2*b.halfSize[2])
}

func (b *box) MarshalJSON() ([]byte, error) {
	config, err := NewGeometryConfig(b)
	if err != nil {
		return nil, err
	}
	return json.Marshal(config)
}

// SetLabel sets the label of this box.
func (b *box) SetLabel(label string) {
	b.label = label
}

// Label returns the label of this box.
func (b *box) Label() string {
	return b.label
}

// Pose returns the pose of the box.
func (b *box) Pose() Pose {
	return b.pose
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
	return PoseAlmostEqualEps(b.pose, other.pose, 1e-6)
}

// Transform premultiplies the box pose with a transform, allowing the box to be moved in space.
func (b *box) Transform(toPremultiply Pose) Geometry {
	return &box{
		pose:            Compose(toPremultiply, b.pose),
		halfSize:        b.halfSize,
		boundingSphereR: b.boundingSphereR,
		label:           b.label,
	}
}

// ToProtobuf converts the box to a Geometry proto message.
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
		Label: b.label,
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
	if other, ok := g.(*capsule); ok {
		return capsuleVsBoxCollision(other, b), nil
	}
	if other, ok := g.(*point); ok {
		return pointVsBoxCollision(other.position, b), nil
	}
	return true, newCollisionTypeUnsupportedError(b, g)
}

func (b *box) DistanceFrom(g Geometry) (float64, error) {
	if other, ok := g.(*box); ok {
		return boxVsBoxDistance(b, other), nil
	}
	if other, ok := g.(*sphere); ok {
		return sphereVsBoxDistance(other, b), nil
	}
	if other, ok := g.(*capsule); ok {
		return capsuleVsBoxDistance(other, b), nil
	}
	if other, ok := g.(*point); ok {
		return pointVsBoxDistance(other.position, b), nil
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
	if other, ok := g.(*capsule); ok {
		return boxInCapsule(b, other), nil
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
func (b *box) pointPenetrationDepth(pt r3.Vector) float64 {
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

// vertices returns the vertices defining the box.
func (b *box) vertices() []r3.Vector {
	verts := make([]r3.Vector, 0, 8)
	for _, vert := range boxVertices {
		offset := NewPoseFromPoint(r3.Vector{X: vert.X * b.halfSize[0], Y: vert.Y * b.halfSize[1], Z: vert.Z * b.halfSize[2]})
		verts = append(verts, Compose(b.pose, offset).Point())
	}
	return verts
}

// vertices returns the vertices defining the box.
func (b *box) toMesh() *mesh {
	if b.mesh == nil {
		m := &mesh{pose: b.pose}
		triangles := make([]*triangle, 0, 12)
		verts := b.vertices()
		for _, tri := range boxTriangles {
			triangles = append(triangles, newTriangle(verts[tri[0]], verts[tri[1]], verts[tri[2]]))
		}
		m.triangles = triangles
		b.mesh = m
	}
	return b.mesh
}

// rotationMatrix returns the cached matrix if it exists, and generates it if not.
func (b *box) rotationMatrix() *RotationMatrix {
	b.once.Do(func() { b.rotMatrix = b.pose.Orientation().RotationMatrix() })

	return b.rotMatrix
}

// boxVsBoxCollision takes two boxes as arguments and returns a bool describing if they are in collision,
// true == collision / false == no collision.
// Since the separating axis test can exit early if no collision is found, it is efficient to avoid calling boxVsBoxDistance.
func boxVsBoxCollision(a, b *box) bool {
	centerDist := b.pose.Point().Sub(a.pose.Point())

	// check if there is a distance between bounding spheres to potentially exit early
	if centerDist.Norm()-(a.boundingSphereR+b.boundingSphereR) > CollisionBuffer {
		return false
	}

	rmA := a.rotationMatrix()
	rmB := b.rotationMatrix()

	for i := 0; i < 3; i++ {
		if separatingAxisTest(centerDist, rmA.Row(i), a.halfSize, b.halfSize, rmA, rmB) > CollisionBuffer {
			return false
		}
		if separatingAxisTest(centerDist, rmB.Row(i), a.halfSize, b.halfSize, rmA, rmB) > CollisionBuffer {
			return false
		}
		for j := 0; j < 3; j++ {
			crossProductPlane := rmA.Row(i).Cross(rmB.Row(j))

			// if edges are parallel, this check is already accounted for by one of the face projections, so skip this case
			if !utils.Float64AlmostEqual(crossProductPlane.Norm(), 0, floatEpsilon) {
				if separatingAxisTest(centerDist, crossProductPlane, a.halfSize, b.halfSize, rmA, rmB) > CollisionBuffer {
					return false
				}
			}
		}
	}
	return true
}

// boxVsBoxDistance takes two boxes as arguments and returns a floating point number.  If this number is nonpositive it represents
// the penetration depth for the two boxes, which are in collision.  If the returned float is positive it represents
// a lower bound on the separation distance for the two boxes, which are not in collision.
// NOTES: calculating the true separation distance is a computationally infeasible problem
//
//	the "minimum translation vector" (MTV) can also be computed here but is not currently as there is no use for it yet
//
// references:  https://comp.graphics.algorithms.narkive.com/jRAgjIUh/obb-obb-distance-calculation
//
//	https://dyn4j.org/2010/01/sat/#sat-nointer
func boxVsBoxDistance(a, b *box) float64 {
	centerDist := b.pose.Point().Sub(a.pose.Point())

	// check if there is a distance between bounding spheres to potentially exit early
	if boundingSphereDist := centerDist.Norm() - a.boundingSphereR - b.boundingSphereR; boundingSphereDist > CollisionBuffer {
		return boundingSphereDist
	}

	rmA := a.rotationMatrix()
	rmB := b.rotationMatrix()

	// iterate over axes of box
	max := math.Inf(-1)
	for i := 0; i < 3; i++ {
		// project onto face of box A
		separation := separatingAxisTest(centerDist, rmA.Row(i), a.halfSize, b.halfSize, rmA, rmB)
		if separation > max {
			max = separation
		}

		// project onto face of box B
		separation = separatingAxisTest(centerDist, rmB.Row(i), a.halfSize, b.halfSize, rmA, rmB)
		if separation > max {
			max = separation
		}

		// project onto a plane created by cross product of two edges from boxes
		for j := 0; j < 3; j++ {
			crossProductPlane := rmA.Row(i).Cross(rmB.Row(j))

			// if edges are parallel, this check is already accounted for by one of the face projections, so skip this case
			if !utils.Float64AlmostEqual(crossProductPlane.Norm(), 0, floatEpsilon) {
				separation = separatingAxisTest(centerDist, crossProductPlane, a.halfSize, b.halfSize, rmA, rmB)
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
	for _, vertex := range inner.vertices() {
		if !pointVsBoxCollision(vertex, outer) {
			return false
		}
	}
	return true
}

// boxInSphere returns a bool describing if the given box is completely encompassed by the given sphere.
func boxInSphere(b *box, s *sphere) bool {
	for _, vertex := range b.vertices() {
		if sphereVsPointDistance(s, vertex) > CollisionBuffer {
			return false
		}
	}
	return sphereVsPointDistance(s, b.pose.Point()) <= 0
}

// boxInCapsule returns a bool describing if the given box is completely encompassed by the given capsule.
func boxInCapsule(b *box, c *capsule) bool {
	for _, vertex := range b.vertices() {
		if capsuleVsPointDistance(c, vertex) > CollisionBuffer {
			return false
		}
	}
	return true
}

// separatingAxisTest projects two boxes onto the given plane and compute how much distance is between them along
// this plane.  Per the separating hyperplane theorem, if such a plane exists (and a positive number is returned)
// this proves that there is no collision between the boxes
// references:  https://gamedev.stackexchange.com/questions/112883/simple-3d-obb-collision-directx9-c
//
//	https://gamedev.stackexchange.com/questions/25397/obb-vs-obb-collision-detection
//	https://www.cs.bgu.ac.il/~vgp192/wiki.files/Separating%20Axis%20Theorem%20for%20Oriented%20Bounding%20Boxes.pdf
//	https://gamedev.stackexchange.com/questions/112883/simple-3d-obb-collision-directx9-c
func separatingAxisTest(positionDelta, plane r3.Vector, halfSizeA, halfSizeB [3]float64, rmA, rmB *RotationMatrix) float64 {
	sum := math.Abs(positionDelta.Dot(plane))
	for i := 0; i < 3; i++ {
		sum -= math.Abs(rmA.Row(i).Mul(halfSizeA[i]).Dot(plane))
		sum -= math.Abs(rmB.Row(i).Mul(halfSizeB[i]).Dot(plane))
	}
	return sum
}

// ToPointCloud converts a box geometry into a []r3.Vector. This method takes one argument which
// determines how many points to place per square mm. If the argument is set to 0. we automatically
// substitute the value with defaultPointDensity.
func (b *box) ToPoints(resolution float64) []r3.Vector {
	// check for user defined spacing
	var iter float64
	if resolution > 0. {
		iter = resolution
	} else {
		iter = defaultPointDensity
	}

	// the boolean values which are passed into the fillFaces method allow for the edges of the
	// box to only be iterated over once. This removes duplicate points.
	// TODO: the fillFaces method calls can be made concurrent if the ToPointCloud method is too slow
	var facePoints []r3.Vector
	facePoints = append(facePoints, fillFaces(b.halfSize, iter, 0, true, false)...)
	facePoints = append(facePoints, fillFaces(b.halfSize, iter, 1, true, true)...)
	facePoints = append(facePoints, fillFaces(b.halfSize, iter, 2, false, false)...)

	transformedVecs := transformPointsToPose(facePoints, b.Pose())
	return transformedVecs
}

// fillFaces returns a list of vectors which lie on the surface of the box.
func fillFaces(halfSize [3]float64, iter float64, fixedDimension int, orEquals1, orEquals2 bool) []r3.Vector {
	var facePoints []r3.Vector
	// create points on box faces with box centered at (0, 0, 0)
	starts := [3]float64{0.0, 0.0, 0.0}
	// depending on which face we want to fill, one of i,j,k is kept constant
	starts[fixedDimension] = halfSize[fixedDimension]
	for i := starts[0]; lessThan(orEquals1, i, halfSize[0]); i += iter {
		for j := starts[1]; lessThan(orEquals2, j, halfSize[1]); j += iter {
			for k := starts[2]; k <= halfSize[2]; k += iter {
				p1 := r3.Vector{i, j, k}
				p2 := r3.Vector{i, j, -k}
				p3 := r3.Vector{i, -j, k}
				p4 := r3.Vector{i, -j, -k}
				p5 := r3.Vector{-i, j, k}
				p6 := r3.Vector{-i, j, -k}
				p7 := r3.Vector{-i, -j, -k}
				p8 := r3.Vector{-i, -j, k}

				switch {
				case i == 0.0 && j == 0.0:
					facePoints = append(facePoints, p1, p2)
				case j == 0.0 && k == 0.0:
					facePoints = append(facePoints, p1, p5)
				case i == 0.0 && k == 0.0:
					facePoints = append(facePoints, p1, p7)
				case i == 0.0:
					facePoints = append(facePoints, p1, p2, p3, p4)
				case j == 0.0:
					facePoints = append(facePoints, p1, p2, p5, p6)
				case k == 0.0:
					facePoints = append(facePoints, p1, p3, p5, p8)
				default:
					facePoints = append(facePoints, p1, p2, p3, p4, p5, p6, p7, p8)
				}
			}
		}
	}
	return facePoints
}

// lessThan checks if v1 <= v1 only if orEquals is true, otherwise we check if v1 < v2.
func lessThan(orEquals bool, v1, v2 float64) bool {
	if orEquals {
		return v1 <= v2
	}
	return v1 < v2
}

// transformPointsToPose gives vectors the proper orientation then translates them to the desired position.
func transformPointsToPose(facePoints []r3.Vector, pose Pose) []r3.Vector {
	var transformedVectors []r3.Vector
	// create pose for a vector at origin from the desired orientation
	originWithPose := NewPoseFromOrientation(pose.Orientation())
	// point at specified offset with (0,0,0,1) axis angles
	identityPose := NewPoseFromPoint(pose.Point())
	// point at specified offset with desired orientation
	offsetBy := Compose(identityPose, originWithPose)
	for i := range facePoints {
		transformedVec := Compose(offsetBy, NewPoseFromPoint(facePoints[i])).Point()
		transformedVectors = append(transformedVectors, transformedVec)
	}
	return transformedVectors
}
