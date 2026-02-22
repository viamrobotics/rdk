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

// The 12 edges of a box, as pairs of vertex indices (vertices differing in exactly one coordinate).
var boxEdgeIndices = [12][2]int{
	{0, 1}, {0, 2}, {0, 4},
	{1, 3}, {1, 5},
	{2, 3}, {2, 6},
	{3, 7},
	{4, 5}, {4, 6},
	{5, 7},
	{6, 7},
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
	center          Pose
	centerPt        r3.Vector
	halfSize        [3]float64
	boundingSphereR float64
	label           string
	mesh            *Mesh
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
		center:          pose,
		centerPt:        pose.Point(),
		halfSize:        [3]float64{halfSize.X, halfSize.Y, halfSize.Z},
		boundingSphereR: halfSize.Norm(),
		label:           label,
	}, nil
}

func (b *box) Hash() int {
	return HashPose(b.center) + int((111*b.halfSize[0])+(222*b.halfSize[1])+(333*b.halfSize[2]))
}

// String returns a human readable string that represents the box.
func (b *box) String() string {
	return fmt.Sprintf("Type: Box | Position: X:%.1f, Y:%.1f, Z:%.1f | Dims: X:%.0f, Y:%.0f, Z:%.0f",
		b.centerPt.X, b.centerPt.Y, b.centerPt.Z, 2*b.halfSize[0], 2*b.halfSize[1], 2*b.halfSize[2])
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
	return b.center
}

// AlmostEqual compares the box with another geometry and checks if they are equivalent.
func (b *box) almostEqual(g Geometry) bool {
	other, ok := g.(*box)
	if !ok {
		return false
	}
	for i := 0; i < 3; i++ {
		if !utils.Float64AlmostEqual(b.halfSize[i], other.halfSize[i], 1e-8) {
			return false
		}
	}
	return PoseAlmostEqualEps(b.center, other.center, 1e-6)
}

// Transform premultiplies the box pose with a transform, allowing the box to be moved in space.
func (b *box) Transform(toPremultiply Pose) Geometry {
	p := Compose(toPremultiply, b.center)
	return &box{
		center:          p,
		centerPt:        p.Point(),
		halfSize:        b.halfSize,
		boundingSphereR: b.boundingSphereR,
		label:           b.label,
	}
}

// ToProtobuf converts the box to a Geometry proto message.
func (b *box) ToProtobuf() *commonpb.Geometry {
	return &commonpb.Geometry{
		Center: PoseToProtobuf(b.center),
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

// CollidesWith checks if the given box collides with the given geometry and returns true if it
// does. If there's no collision, the method will return the distance between the box and input
// geometry. If there is a collision, a negative number is returned.
func (b *box) CollidesWith(g Geometry, collisionBufferMM float64) (bool, float64, error) {
	switch other := g.(type) {
	case *Mesh:
		return other.CollidesWith(b, collisionBufferMM)
	case *box:
		c, d := boxVsBoxCollision(b, other, collisionBufferMM)
		if c {
			return true, -1, nil
		}
		return false, d, nil
	case *sphere:
		col, dist := sphereVsBoxCollision(other, b, collisionBufferMM)
		if col {
			return true, -1, nil
		}
		return false, dist, nil
	case *capsule:
		col, d := capsuleVsBoxCollision(other, b, collisionBufferMM)
		return col, d, nil
	case *point:
		col, d := pointVsBoxCollision(other.position, b, collisionBufferMM)
		return col, d, nil
	default:
		return true, collisionBufferMM, newCollisionTypeUnsupportedError(b, g)
	}
}

func (b *box) DistanceFrom(g Geometry) (float64, error) {
	switch other := g.(type) {
	case *Mesh:
		return other.DistanceFrom(b)
	case *box:
		return boxVsBoxDistance(b, other), nil
	case *sphere:
		return sphereVsBoxDistance(other, b), nil
	case *capsule:
		return capsuleVsBoxDistance(other, b), nil
	case *point:
		return pointVsBoxDistance(other.position, b), nil
	default:
		return math.Inf(-1), newCollisionTypeUnsupportedError(b, g)
	}
}

func (b *box) EncompassedBy(g Geometry) (bool, error) {
	switch other := g.(type) {
	case *Mesh:
		return false, nil // Like points, meshes have no volume and cannot encompass
	case *box:
		return boxInBox(b, other), nil
	case *sphere:
		return boxInSphere(b, other), nil
	case *capsule:
		return boxInCapsule(b, other), nil
	case *point:
		return false, nil
	default:
		return false, newCollisionTypeUnsupportedError(b, g)
	}
}

// closestPoint returns the closest point on the specified box to the specified point
// Reference: https://github.com/gszauer/GamePhysicsCookbook/blob/a0b8ee0c39fed6d4b90bb6d2195004dfcf5a1115/Code/Geometry3D.cpp#L165
func (b *box) closestPoint(pt r3.Vector) r3.Vector {
	result := b.centerPt
	direction := pt.Sub(result)
	rm := b.center.Orientation().RotationMatrix()
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
	direction := pt.Sub(b.centerPt)
	rm := b.center.Orientation().RotationMatrix()
	//nolint: revive
	min := math.Inf(1)
	for i := 0; i < 3; i++ {
		axis := rm.Row(i)
		projection := direction.Dot(axis)
		if distance := math.Abs(projection - b.halfSize[i]); distance < min {
			//nolint: revive
			min = distance
		}
		if distance := math.Abs(projection + b.halfSize[i]); distance < min {
			//nolint: revive
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
		verts = append(verts, Compose(b.center, offset).Point())
	}
	return verts
}

// toMesh returns a 12-triangle mesh representation of the box, 2 right triangles for each face.
func (b *box) toMesh() *Mesh {
	if b.mesh == nil {
		m := &Mesh{pose: NewZeroPose()}
		triangles := make([]*Triangle, 0, 12)
		verts := b.vertices()
		for _, tri := range boxTriangles {
			triangles = append(triangles, NewTriangle(verts[tri[0]], verts[tri[1]], verts[tri[2]]))
		}
		m.triangles = triangles
		// bvh is built lazily via ensureBVH() on first collision check
		b.mesh = m
	}
	return b.mesh
}

// rotationMatrix returns the cached matrix if it exists, and generates it if not.
func (b *box) rotationMatrix() *RotationMatrix {
	b.once.Do(func() { b.rotMatrix = b.center.Orientation().RotationMatrix() })

	return b.rotMatrix
}

// boxVsBoxCollision takes two boxes as arguments and returns a bool describing if they are in collision,
// true == collision / false == no collision.
// Since the separating axis test can exit early if no collision is found, it is efficient to avoid calling boxVsBoxDistance.
func boxVsBoxCollision(a, b *box, collisionBufferMM float64) (bool, float64) {
	//~ return boxVsBoxGJKCollision(a, b, collisionBufferMM)
	centerDist := b.centerPt.Sub(a.centerPt)

	// check if there is a distance between bounding spheres to potentially exit early
	dist := centerDist.Norm() - (a.boundingSphereR + b.boundingSphereR)
	if dist > collisionBufferMM {
		return false, dist
	}

	rmA := a.rotationMatrix()
	rmB := b.rotationMatrix()

	for i := 0; i < 3; i++ {
		dist = separatingAxisTest(centerDist, rmA.Row(i), a.halfSize, b.halfSize, rmA, rmB)
		if dist > collisionBufferMM {
			return false, dist
		}
		dist = separatingAxisTest(centerDist, rmB.Row(i), a.halfSize, b.halfSize, rmA, rmB)
		if dist > collisionBufferMM {
			return false, dist
		}
		for j := 0; j < 3; j++ {
			crossProductPlane := rmA.Row(i).Cross(rmB.Row(j))

			// if edges are parallel, this check is already accounted for by one of the face projections, so skip this case
			if !utils.Float64AlmostEqual(crossProductPlane.Norm(), 0, floatEpsilon) {
				dist = separatingAxisTest(centerDist, crossProductPlane.Normalize(), a.halfSize, b.halfSize, rmA, rmB)
				if dist > collisionBufferMM {
					return false, dist
				}
			}
		}
	}
	return true, -1
}

// boxVsBoxGJKCollision uses GJK with early exit for collision detection with a buffer.
// At each iteration the support function yields a lower bound on the true distance that
// monotonically increases; as soon as this bound exceeds collisionBufferMM the function
// exits with no-collision. When GJK fully converges the returned distance is exact.
//
// Returns (colliding, distance) where distance is:
//   - exact Euclidean distance when GJK fully converges
//   - a tight lower bound when early-exiting (distance clearly exceeds buffer)
//   - -1 when colliding (penetration depth not computed)
func boxVsBoxGJKCollision(a, b *box, collisionBufferMM float64) (bool, float64) {
	centerDist := b.centerPt.Sub(a.centerPt)

	// Bounding-sphere pre-check (same as SAT path).
	dist := centerDist.Norm() - (a.boundingSphereR + b.boundingSphereR)
	if dist > collisionBufferMM {
		return false, dist
	}

	d := centerDist
	if d.Norm2() < floatEpsilon*floatEpsilon {
		d = r3.Vector{X: 1}
	}

	w := gjkMinkowskiSupport(a, b, d)
	simplex := []r3.Vector{w}
	v := w
	mu := 0.0 // best lower bound on distance

	const maxIter = 64
	const eps = 1e-10

	for iter := 0; iter < maxIter; iter++ {
		vv := v.Norm2()
		if vv < 1e-20 {
			return true, -1
		}
		vNorm := math.Sqrt(vv)

		d = v.Mul(-1)
		w = gjkMinkowskiSupport(a, b, d)

		// Update lower bound: all points in the Minkowski difference satisfy
		// x·(v/||v||) >= w·v/||v||, so the closest point is at least this far.
		if lb := v.Dot(w) / vNorm; lb > mu {
			mu = lb
		}

		// Early exit: lower bound proves distance exceeds buffer.
		if mu > collisionBufferMM {
			return false, mu
		}

		// Convergence: new support point can't improve distance significantly.
		if vv-v.Dot(w) <= eps*vv {
			break
		}

		simplex = append(simplex, w)
		switch len(simplex) {
		case 2:
			v, simplex = gjkClosestOnSegment(simplex[0], simplex[1])
		case 3:
			v, simplex = gjkClosestOnTriangle(simplex[0], simplex[1], simplex[2])
		case 4:
			v, simplex = gjkClosestOnTetrahedron(simplex)
		}
	}

	// Fully converged: v.Norm() is the exact distance.
	finalDist := v.Norm()
	if finalDist > collisionBufferMM {
		return false, finalDist
	}
	return true, -1
}

// boxVsBoxDistance takes two boxes as arguments and returns a floating point number. If this number is nonpositive it represents
// the penetration depth for the two boxes, which are in collision. If the returned float is positive, it is the
// separation distance for the two boxes, which are not in collision.
// For penetration depth, the SAT (Separating Axis Theorem) is used which gives the minimum translation to separate.
// For separation distance, alternating closest-point projections are used which give the exact Euclidean distance.
//
// references:  https://comp.graphics.algorithms.narkive.com/jRAgjIUh/obb-obb-distance-calculation
//
//	https://dyn4j.org/2010/01/sat/#sat-nointer
func boxVsBoxDistance(a, b *box) float64 {
	_, max := boxVsBoxCollision(a, b, 0)

	// If the boxes are colliding, return the SAT penetration depth.
	if max <= 0 {
		return max
	}

	// For non-colliding boxes, compute exact distance by enumerating closest
	// vertex-to-box and edge-to-edge distances across both boxes.
	return boxVsBoxSeparationDist(a, b)
}

// boxVsBoxSeparationDist computes the exact Euclidean distance between two non-colliding boxes
// by checking all vertex-to-box and edge-to-edge feature pairs.
func boxVsBoxSeparationDist(a, b *box) float64 {
	vertsA := a.vertices()
	vertsB := b.vertices()

	minDist := math.Inf(1)

	// Check each vertex of A against closest point on B, and vice versa.
	for i := range vertsA {
		if d := vertsA[i].Sub(b.closestPoint(vertsA[i])).Norm(); d < minDist {
			minDist = d
		}
	}
	for i := range vertsB {
		if d := vertsB[i].Sub(a.closestPoint(vertsB[i])).Norm(); d < minDist {
			minDist = d
		}
	}

	// Check all edge-edge pairs for edge-to-edge closest distance.
	for _, ea := range boxEdgeIndices {
		for _, eb := range boxEdgeIndices {
			if d := SegmentDistanceToSegment(vertsA[ea[0]], vertsA[ea[1]], vertsB[eb[0]], vertsB[eb[1]]); d < minDist {
				minDist = d
			}
		}
	}

	return minDist
}

// boxVsBoxSATMaxDistance computes the maximum separation gap across all 15 SAT axes
// for two oriented bounding boxes. This value is:
//   - A tight lower bound on the true Euclidean distance between the boxes
//   - Exact when the closest features are face-vertex or face-face
//   - An underestimate only in edge-edge closest-feature configurations
//   - Negative (and equal to penetration depth) when the boxes overlap
func boxVsBoxSATMaxDistance(a, b *box) float64 {
	centerDist := b.centerPt.Sub(a.centerPt)
	rmA := a.rotationMatrix()
	rmB := b.rotationMatrix()
	var input [27]float64
	copy(input[0:9], rmA.mat[:])
	copy(input[9:18], rmB.mat[:])
	copy(input[18:21], a.halfSize[:])
	copy(input[21:24], b.halfSize[:])
	input[24], input[25], input[26] = centerDist.X, centerDist.Y, centerDist.Z
	return obbSATMaxGap(&input)
}

// gjkBoxSupport returns the support point (farthest vertex) of a box in the given direction.
func gjkBoxSupport(b *box, d r3.Vector) r3.Vector {
	rm := b.rotationMatrix()
	result := b.centerPt
	for i := 0; i < 3; i++ {
		axis := rm.Row(i)
		if d.Dot(axis) >= 0 {
			result = result.Add(axis.Mul(b.halfSize[i]))
		} else {
			result = result.Sub(axis.Mul(b.halfSize[i]))
		}
	}
	return result
}

// gjkMinkowskiSupport returns support_A(d) - support_B(-d), a support point
// of the Minkowski difference A - B in direction d.
func gjkMinkowskiSupport(a, b *box, d r3.Vector) r3.Vector {
	return gjkBoxSupport(a, d).Sub(gjkBoxSupport(b, d.Mul(-1)))
}

// gjkClosestOnSegment returns the closest point on segment [a,b] to the origin,
// along with the reduced simplex.
func gjkClosestOnSegment(a, b r3.Vector) (r3.Vector, []r3.Vector) {
	ab := b.Sub(a)
	denom := ab.Norm2()
	if denom < 1e-30 {
		return a, []r3.Vector{a}
	}
	t := a.Mul(-1).Dot(ab) / denom
	if t <= 0 {
		return a, []r3.Vector{a}
	}
	if t >= 1 {
		return b, []r3.Vector{b}
	}
	return a.Add(ab.Mul(t)), []r3.Vector{a, b}
}

// gjkClosestOnTriangle returns the closest point on triangle [a,b,c] to the origin,
// along with the reduced simplex. Uses Ericson's Voronoi region method from
// "Real-Time Collision Detection".
func gjkClosestOnTriangle(a, b, c r3.Vector) (r3.Vector, []r3.Vector) {
	ab := b.Sub(a)
	ac := c.Sub(a)
	ao := a.Mul(-1)

	d1 := ab.Dot(ao)
	d2 := ac.Dot(ao)
	if d1 <= 0 && d2 <= 0 {
		return a, []r3.Vector{a}
	}

	bo := b.Mul(-1)
	d3 := ab.Dot(bo)
	d4 := ac.Dot(bo)
	if d3 >= 0 && d4 <= d3 {
		return b, []r3.Vector{b}
	}

	vc := d1*d4 - d3*d2
	if vc <= 0 && d1 >= 0 && d3 <= 0 {
		v := d1 / (d1 - d3)
		return a.Add(ab.Mul(v)), []r3.Vector{a, b}
	}

	co := c.Mul(-1)
	d5 := ab.Dot(co)
	d6 := ac.Dot(co)
	if d6 >= 0 && d5 <= d6 {
		return c, []r3.Vector{c}
	}

	vb := d5*d2 - d1*d6
	if vb <= 0 && d2 >= 0 && d6 <= 0 {
		w := d2 / (d2 - d6)
		return a.Add(ac.Mul(w)), []r3.Vector{a, c}
	}

	va := d3*d6 - d5*d4
	if va <= 0 && (d4-d3) >= 0 && (d5-d6) >= 0 {
		w := (d4 - d3) / ((d4 - d3) + (d5 - d6))
		return b.Add(c.Sub(b).Mul(w)), []r3.Vector{b, c}
	}

	denom := 1.0 / (va + vb + vc)
	v := vb * denom
	w := vc * denom
	return a.Add(ab.Mul(v)).Add(ac.Mul(w)), []r3.Vector{a, b, c}
}

// gjkOriginInTetrahedron checks whether the origin is inside the tetrahedron
// defined by the four given points, by verifying the origin is on the interior
// side of every face.
func gjkOriginInTetrahedron(pts []r3.Vector) bool {
	type face struct{ v0, v1, v2, opp int }
	faces := [4]face{
		{0, 1, 2, 3},
		{0, 1, 3, 2},
		{0, 2, 3, 1},
		{1, 2, 3, 0},
	}
	for _, f := range faces {
		p0, p1, p2 := pts[f.v0], pts[f.v1], pts[f.v2]
		normal := p1.Sub(p0).Cross(p2.Sub(p0))
		dOrigin := normal.Dot(p0.Mul(-1))
		dOpp := normal.Dot(pts[f.opp].Sub(p0))
		if dOrigin*dOpp < 0 {
			return false
		}
	}
	return true
}

// gjkClosestOnTetrahedron returns the closest point on the tetrahedron to the origin.
// If the origin is inside, returns the zero vector (collision detected).
func gjkClosestOnTetrahedron(pts []r3.Vector) (r3.Vector, []r3.Vector) {
	if gjkOriginInTetrahedron(pts) {
		return r3.Vector{}, pts
	}
	faces := [4][3]int{{0, 1, 2}, {0, 1, 3}, {0, 2, 3}, {1, 2, 3}}
	bestDist := math.Inf(1)
	var bestV r3.Vector
	var bestS []r3.Vector

	for _, f := range faces {
		v, s := gjkClosestOnTriangle(pts[f[0]], pts[f[1]], pts[f[2]])
		if d := v.Norm2(); d < bestDist {
			bestDist = d
			bestV = v
			bestS = s
		}
	}
	return bestV, bestS
}

// boxVsBoxGJKDistance computes the exact Euclidean distance between two boxes
// using the GJK (Gilbert-Johnson-Keerthi) algorithm. Returns 0 for colliding boxes.
// Typically converges in 3-5 iterations for boxes.
func boxVsBoxGJKDistance(a, b *box) float64 {
	return boxVsBoxGJKDistanceSeeded(a, b, b.centerPt.Sub(a.centerPt))
}

// boxVsBoxGJKDistanceSeeded is GJK with a caller-supplied initial search direction.
// A good seed (e.g. the SAT winning axis) can reduce iterations from ~6 to ~1.
func boxVsBoxGJKDistanceSeeded(a, b *box, initialDir r3.Vector) float64 {
	d := initialDir
	if d.Norm2() < floatEpsilon*floatEpsilon {
		d = r3.Vector{X: 1}
	}

	w := gjkMinkowskiSupport(a, b, d)
	simplex := []r3.Vector{w}
	v := w

	const maxIter = 64
	const eps = 1e-10

	for iter := 0; iter < maxIter; iter++ {
		vv := v.Norm2()
		if vv < 1e-20 {
			return 0
		}

		d = v.Mul(-1)
		w = gjkMinkowskiSupport(a, b, d)

		if vv-v.Dot(w) <= eps*vv {
			break
		}

		simplex = append(simplex, w)
		switch len(simplex) {
		case 2:
			v, simplex = gjkClosestOnSegment(simplex[0], simplex[1])
		case 3:
			v, simplex = gjkClosestOnTriangle(simplex[0], simplex[1], simplex[2])
		case 4:
			v, simplex = gjkClosestOnTetrahedron(simplex)
		}
	}

	return v.Norm()
}

// boxVsBoxHybridDistance uses SAT to detect face-separated boxes (returning the exact
// SAT gap in ~200ns) and falls back to GJK seeded with the SAT axis for all other cases.
//
// Detection rule: if the winning SAT axis is a face normal from box X, and box X's other
// two face normals both show overlap (gap <= 0), the boxes are separated only along that
// face and the SAT gap is the exact Euclidean distance. Otherwise GJK refines.
func boxVsBoxHybridDistance(a, b *box) float64 {
	centerDist := b.centerPt.Sub(a.centerPt)
	rmA := a.rotationMatrix()
	rmB := b.rotationMatrix()

	var faceGapsA, faceGapsB [3]float64
	best := math.Inf(-1)
	var bestAxis r3.Vector
	bestFromFaceA := -1 // index into faceGapsA, or -1
	bestFromFaceB := -1

	for i := 0; i < 3; i++ {
		axisA := rmA.Row(i)
		gA := separatingAxisTest(centerDist, axisA, a.halfSize, b.halfSize, rmA, rmB)
		faceGapsA[i] = gA
		if gA > best {
			best = gA
			bestAxis = axisA
			bestFromFaceA = i
			bestFromFaceB = -1
		}

		axisB := rmB.Row(i)
		gB := separatingAxisTest(centerDist, axisB, a.halfSize, b.halfSize, rmA, rmB)
		faceGapsB[i] = gB
		if gB > best {
			best = gB
			bestAxis = axisB
			bestFromFaceA = -1
			bestFromFaceB = i
		}

		for j := 0; j < 3; j++ {
			cp := rmA.Row(i).Cross(rmB.Row(j))
			if !utils.Float64AlmostEqual(cp.Norm(), 0, floatEpsilon) {
				cpn := cp.Normalize()
				if g := separatingAxisTest(centerDist, cpn, a.halfSize, b.halfSize, rmA, rmB); g > best {
					best = g
					bestAxis = cpn
					bestFromFaceA = -1
					bestFromFaceB = -1
				}
			}
		}
	}

	if best <= 0 {
		return best // collision: return SAT penetration depth
	}

	// If winning axis is a face normal and the other two face axes from the
	// same box overlap, the SAT gap IS the exact Euclidean distance.
	if bestFromFaceA >= 0 {
		exact := true
		for i := 0; i < 3; i++ {
			if i != bestFromFaceA && faceGapsA[i] > 0 {
				exact = false
				break
			}
		}
		if exact {
			return best
		}
	}
	if bestFromFaceB >= 0 {
		exact := true
		for i := 0; i < 3; i++ {
			if i != bestFromFaceB && faceGapsB[i] > 0 {
				exact = false
				break
			}
		}
		if exact {
			return best
		}
	}

	// SAT not proven exact; refine with GJK seeded from the SAT axis.
	return boxVsBoxGJKDistanceSeeded(a, b, bestAxis)
}

// boxInBox returns a bool describing if the inner box is completely encompassed by the outer box.
func boxInBox(inner, outer *box) bool {
	for _, vertex := range inner.vertices() {
		c, _ := pointVsBoxCollision(vertex, outer, defaultCollisionBufferMM)
		if !c {
			return false
		}
	}
	return true
}

// boxInSphere returns a bool describing if the given box is completely encompassed by the given sphere.
func boxInSphere(b *box, s *sphere) bool {
	for _, vertex := range b.vertices() {
		if sphereVsPointDistance(s, vertex) > defaultCollisionBufferMM {
			return false
		}
	}
	return sphereVsPointDistance(s, b.centerPt) <= 0
}

// boxInCapsule returns a bool describing if the given box is completely encompassed by the given capsule.
func boxInCapsule(b *box, c *capsule) bool {
	for _, vertex := range b.vertices() {
		if capsuleVsPointDistance(c, vertex) > defaultCollisionBufferMM {
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
