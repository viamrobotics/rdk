package spatialmath

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/chenzhekl/goply"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/encoding/protojson"

	"go.viam.com/rdk/utils"
)

// This file incorporates work covered by the Brax project -- https://github.com/google/brax/blob/main/LICENSE.
// Copyright 2021 The Brax Authors, which is licensed under the Apache License Version 2.0 (the "License").
// You may obtain a copy of the license at http://www.apache.org/licenses/LICENSE-2.0.

// The set of supported mesh file types.
type meshType string

const plyType = meshType("ply")

// Mesh is a set of triangles at some pose. Triangle points are in the frame of the mesh.
type Mesh struct {
	pose      Pose
	triangles []*Triangle
	label     string

	// information used for encoding to protobuf
	fileType meshType
	rawBytes []byte
}

// NewMesh creates a mesh from the given triangles and pose.
func NewMesh(pose Pose, triangles []*Triangle, label string) *Mesh {
	mesh := &Mesh{
		pose:      pose,
		triangles: triangles,
		label:     label,
	}

	// Convert triangles to PLY for protobuf
	plyBytes := mesh.TrianglesToPLYBytes(false) // Keep it in the local frame
	mesh.fileType = plyType
	mesh.rawBytes = plyBytes

	return mesh
}

// NewMeshFromPLYFile is a helper function to create a Mesh geometry from a PLY file.
func NewMeshFromPLYFile(path string) (*Mesh, error) {
	//nolint:gosec
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	//nolint:errcheck
	defer file.Close()
	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return newMeshFromBytes(NewZeroPose(), bytes, path)
}

func newMeshFromBytes(pose Pose, data []byte, label string) (mesh *Mesh, err error) {
	// the library we are using for PLY parsing is fragile, so
	defer func() {
		if r := recover(); r != nil {
			err = errors.Wrap(errors.Errorf("%v", r), "error reading mesh")
		}
	}()

	ply := goply.New(bytes.NewReader(data))
	vertices := ply.Elements("vertex")
	faces := ply.Elements("face")
	triangles := make([]*Triangle, 0)
	for _, face := range faces {
		pts := []r3.Vector{}
		idxIface := face["vertex_indices"]
		for _, i := range idxIface.([]any) {
			x, err := cast.ToFloat64E(vertices[cast.ToInt(i)]["x"])
			if err != nil {
				return nil, err
			}
			y, err := cast.ToFloat64E(vertices[cast.ToInt(i)]["y"])
			if err != nil {
				return nil, err
			}
			z, err := cast.ToFloat64E(vertices[cast.ToInt(i)]["z"])
			if err != nil {
				return nil, err
			}
			pts = append(pts, r3.Vector{X: x * 1000, Y: y * 1000, Z: z * 1000})
		}
		if len(pts) != 3 {
			return nil, errors.New("triangle did not have three points")
		}
		tri := NewTriangle(pts[0], pts[1], pts[2])
		triangles = append(triangles, tri)
	}
	return &Mesh{
		pose:      pose,
		triangles: triangles,
		label:     label,
		fileType:  plyType,
		rawBytes:  data,
	}, nil
}

// NewMeshFromProto creates a new mesh from a protobuf mesh.
func NewMeshFromProto(pose Pose, m *commonpb.Mesh, label string) (*Mesh, error) {
	switch m.ContentType {
	case string(plyType):
		return newMeshFromBytes(pose, m.Mesh, label)
	default:
		return nil, fmt.Errorf("unsupported Mesh type: %s", m.ContentType)
	}
}

// String returns a human readable string that represents the box.
func (m *Mesh) String() string {
	return fmt.Sprintf("Type: Mesh | Position: X:%.1f, Y:%.1f, Z:%.1f | Triangle count: %d",
		m.pose.Point().X, m.pose.Point().Y, m.pose.Point().Z, len(m.triangles))
}

// ToProtobuf converts a Mesh to its protobuf representation.
// Note that if the mesh's rawBytes and fileType fields are unset this will result in a malformed message.
func (m *Mesh) ToProtobuf() *commonpb.Geometry {
	return &commonpb.Geometry{
		Center: PoseToProtobuf(m.pose),
		GeometryType: &commonpb.Geometry_Mesh{
			Mesh: &commonpb.Mesh{
				ContentType: string(m.fileType),
				Mesh:        m.rawBytes,
			},
		},
		Label: m.label,
	}
}

// Pose returns the pose of the mesh.
func (m *Mesh) Pose() Pose {
	return m.pose
}

// Triangles returns the triangles associated with the mesh.
func (m *Mesh) Triangles() []*Triangle {
	return m.triangles
}

// Transform transforms the mesh. As triangles are in the mesh's frame, they are unchanged.
func (m *Mesh) Transform(pose Pose) Geometry {
	// Triangle points are in frame of mesh, like the corners of a box, so no need to transform them
	return &Mesh{
		pose:      Compose(pose, m.pose),
		triangles: m.triangles,
		label:     m.label,
		fileType:  m.fileType,
		rawBytes:  m.rawBytes,
	}
}

// CollidesWith checks if the given mesh collides with the given geometry and returns true if it does.
func (m *Mesh) CollidesWith(g Geometry, collisionBufferMM float64) (bool, error) {
	switch other := g.(type) {
	case *box:
		// Mesh-ifying the box misses the case where the box encompasses a mesh triangle without its surface intersecting a triangle.
		encompassed := m.boxIntersectsVertex(other)
		if encompassed {
			return true, nil
		}
		// Convert box to mesh and check triangle collisions
		return m.collidesWithMesh(other.toMesh(), collisionBufferMM), nil
	case *capsule:
		// Use existing capsule vs mesh distance check
		// TODO: This is inefficient! Replace with a function with a short-circuit.
		dist := capsuleVsMeshDistance(other, m)
		return dist <= collisionBufferMM, nil
	case *point:
		return m.collidesWithSphere(&sphere{pose: NewPoseFromPoint(other.position)}, collisionBufferMM), nil
	case *sphere:
		return m.collidesWithSphere(other, collisionBufferMM), nil
	case *Mesh:
		return m.collidesWithMesh(other, collisionBufferMM), nil
	default:
		return true, newCollisionTypeUnsupportedError(m, g)
	}
}

// EncompassedBy returns whether this mesh is completely contained within another geometry.
func (m *Mesh) EncompassedBy(g Geometry) (bool, error) {
	if _, ok := g.(*point); ok {
		return false, nil
	}
	if _, ok := g.(*Mesh); ok {
		return false, nil
	}
	// For all other geometry types, check if all vertices of all triangles are inside
	for _, pt := range m.ToPoints(1) {
		collides, err := NewPoint(pt, "").CollidesWith(g, defaultCollisionBufferMM)
		if err != nil {
			return false, err
		}
		if !collides {
			return false, nil
		}
	}
	return true, nil
}

// DistanceFrom returns the minimum distance between this mesh and another geometry.
func (m *Mesh) DistanceFrom(g Geometry) (float64, error) {
	switch other := g.(type) {
	case *box:
		// Mesh-ifying the box misses the case where the box encompasses a mesh triangle without its surface intersecting a triangle.
		encompassed := m.boxIntersectsVertex(other)
		if encompassed {
			return 0, nil
		}
		return m.distanceFromMesh(other.toMesh()), nil
	case *capsule:
		return capsuleVsMeshDistance(other, m), nil
	case *point:
		return m.distanceFromSphere(&sphere{pose: NewPoseFromPoint(other.position)}), nil
	case *sphere:
		return m.distanceFromSphere(other), nil
	case *Mesh:
		return m.distanceFromMesh(other), nil
	default:
		return math.Inf(-1), newCollisionTypeUnsupportedError(m, g)
	}
}

// Returns true if any triangle vertex of the mesh intersects the box.
func (m *Mesh) boxIntersectsVertex(b *box) bool {
	// Use map to deduplicate vertices
	pointMap := make(map[string]r3.Vector)
	// Add all triangle vertices, formatting as a string for map deduplication
	for _, tri := range m.triangles {
		for _, pt := range tri.Points() {
			// If this is a shared vertex we can skip the math after the first time
			key := fmt.Sprintf("%.10f,%.10f,%.10f", pt.X, pt.Y, pt.Z)
			if _, ok := pointMap[key]; ok {
				continue
			}
			pointMap[key] = pt
			worldPt := Compose(m.pose, NewPoseFromPoint(pt)).Point()
			if pointVsBoxCollision(worldPt, b, defaultCollisionBufferMM) {
				return true
			}
		}
	}
	return false
}

func (m *Mesh) distanceFromSphere(s *sphere) float64 {
	pt := s.pose.Point()
	minDist := math.Inf(1)
	// Transform all triangles to world space once
	for _, tri := range m.triangles {
		closestPt := ClosestPointTrianglePoint(tri.Transform(m.pose), pt)
		dist := closestPt.Sub(pt).Norm() - s.radius
		if dist < minDist {
			minDist = dist
		}
	}
	return minDist
}

func (m *Mesh) collidesWithSphere(s *sphere, buffer float64) bool {
	pt := s.pose.Point()
	// Transform all triangles to world space once
	for _, tri := range m.triangles {
		closestPt := ClosestPointTrianglePoint(tri.Transform(m.pose), pt)
		if closestPt.Sub(pt).Norm() <= s.radius+buffer {
			return true
		}
	}
	return false
}

// collidesWithMesh checks if this mesh collides with another mesh
// TODO: This function is *begging* for GPU acceleration.
func (m *Mesh) collidesWithMesh(other *Mesh, collisionBufferMM float64) bool {
	// Transform all triangles to world space
	worldTris1 := make([]*Triangle, len(m.triangles))
	for i, tri := range m.triangles {
		worldTris1[i] = tri.Transform(m.pose)
	}
	worldTris2 := make([]*Triangle, len(other.triangles))
	for i, tri := range other.triangles {
		worldTris2[i] = tri.Transform(other.pose)
	}

	// Check if any triangles from either mesh collide.
	// If two triangles intersect, then the segment between two vertices of one triangle intersects the other triangle.
	for _, worldTri1 := range worldTris1 {
		p1 := worldTri1.Points()

		for _, worldTri2 := range worldTris2 {
			p2 := worldTri2.Points()

			// Check segments from tri1 against tri2
			for i := 0; i < 3; i++ {
				start := p1[i]
				end := p1[(i+1)%3]
				bestSegPt, bestTriPt := ClosestPointsSegmentTriangle(start, end, worldTri2)
				if bestSegPt.Sub(bestTriPt).Norm() <= collisionBufferMM {
					return true
				}
			}

			// Check segments from tri2 against tri1
			for i := 0; i < 3; i++ {
				start := p2[i]
				end := p2[(i+1)%3]
				bestSegPt, bestTriPt := ClosestPointsSegmentTriangle(start, end, worldTri1)
				if bestSegPt.Sub(bestTriPt).Norm() <= collisionBufferMM {
					return true
				}
			}
		}
	}
	return false
}

// distanceFromMesh returns the minimum distance between this mesh and another mesh.
func (m *Mesh) distanceFromMesh(other *Mesh) float64 {
	// Transform all triangles to world space
	worldTris1 := make([]*Triangle, len(m.triangles))
	for i, tri := range m.triangles {
		worldTris1[i] = tri.Transform(m.pose)
	}

	worldTris2 := make([]*Triangle, len(other.triangles))
	for i, tri := range other.triangles {
		worldTris2[i] = tri.Transform(other.pose)
	}

	minDist := math.Inf(1)
	for _, worldTri1 := range worldTris1 {
		p1 := worldTri1.Points()

		for _, worldTri2 := range worldTris2 {
			p2 := worldTri2.Points()

			// Check segments from tri1 against tri2
			for i := 0; i < 3; i++ {
				start := p1[i]
				end := p1[(i+1)%3]
				bestSegPt, bestTriPt := ClosestPointsSegmentTriangle(start, end, worldTri2)
				dist := bestSegPt.Sub(bestTriPt).Norm()
				if dist < minDist {
					minDist = dist
				}
			}

			// Check segments from tri2 against tri1
			for i := 0; i < 3; i++ {
				start := p2[i]
				end := p2[(i+1)%3]
				bestSegPt, bestTriPt := ClosestPointsSegmentTriangle(start, end, worldTri1)
				dist := bestSegPt.Sub(bestTriPt).Norm()
				if dist < minDist {
					minDist = dist
				}
			}
		}
	}
	return minDist
}

// SetLabel sets the name of the mesh.
func (m *Mesh) SetLabel(label string) {
	m.label = label
}

// Label returns the name of the mesh.
func (m *Mesh) Label() string {
	return m.label
}

// ToPoints returns a vector of points that together represent a point cloud of the Mesh.
// This method takes one argument which  determines how many points to place per square mm.
// If the argument is set to 0. we automatically substitute the value with defaultPointDensity.
func (m *Mesh) ToPoints(density float64) []r3.Vector {
	if density == 0 {
		density = defaultPointDensity // defaultPointDensity is currently 0, so this isn't doing anything
		// But this is consistent with the use of density/resolution in other geometries (see box.go)
	}

	// Use map to deduplicate vertices
	pointMap := make(map[string]r3.Vector)

	// Add all points, formatting as a string for map deduplication
	for _, tri := range m.triangles {
		triPts := tri.Points()
		baseLen := 0.
		var baseP0, baseP1, vertex r3.Vector
		for i := range 3 {
			p0 := triPts[i]
			p1 := triPts[(i+1)%3]
			p2 := triPts[(i+2)%3]
			edgeLen := p0.Sub(p1).Norm()
			if edgeLen >= baseLen { // checking >= instead of > accounts for edge case p0=p1=p2
				baseLen = edgeLen
				baseP0 = p0
				baseP1 = p1
				vertex = p2
			}
		}
		// If we have density points per mm^2, we have 1 point per 1/density mm^2
		// We achieve this by tiling each mesh triangle with mini similar triangles whose edge lengths are <= 1/density
		// We choose a miniBaseCount such that we have side length <= 1/density and can fit an integer # of tiles
		// If density = 0, we just take the triangle vertices
		miniBaseCount := max(int(math.Ceil(baseLen*density)), 1)
		rowVec := vertex.Sub(baseP0).Mul(1.0 / float64(miniBaseCount)) // runs in column direction towards vertex
		colVec := baseP1.Sub(baseP0).Mul(1.0 / float64(miniBaseCount)) // runs in row direction towards baseP1

		for row := range miniBaseCount + 1 {
			for col := range miniBaseCount + 1 - row {
				pt := rowVec.Mul(float64(row)).Add(colVec.Mul(float64(col))).Add(baseP0)
				worldPt := Compose(m.pose, NewPoseFromPoint(pt)).Point()
				key := fmt.Sprintf("%.10f,%.10f,%.10f", worldPt.X, worldPt.Y, worldPt.Z)
				pointMap[key] = worldPt
			}
		}
	}

	// Convert map back to slice
	points := make([]r3.Vector, 0, len(pointMap))
	for _, pt := range pointMap {
		points = append(points, pt)
	}
	return points
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (m *Mesh) UnmarshalJSON(data []byte) error {
	var g commonpb.Geometry
	if err := protojson.Unmarshal(data, &g); err != nil {
		return err
	}
	if _, ok := g.GeometryType.(*commonpb.Geometry_Mesh); !ok {
		return errors.New("geometry is not a mesh")
	}

	pose := NewPoseFromProtobuf(g.GetCenter())
	mesh, err := NewMeshFromProto(pose, g.GetMesh(), g.Label)
	if err != nil {
		return err
	}
	*m = *mesh
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (m *Mesh) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(m.ToProtobuf())
}

// MeshBoxIntersectionArea calculates the summed area of all triangles in a mesh
// that intersect with a box geometry and returns the total intersection area.
func MeshBoxIntersectionArea(mesh, theBox Geometry) (float64, error) {
	m, err := utils.AssertType[*Mesh](mesh)
	if err != nil {
		return -1, err
	}
	b, err := utils.AssertType[*box](theBox)
	if err != nil {
		return -1, err
	}

	// Sum the intersection area for each triangle
	totalArea := 0.0
	for _, tri := range m.triangles {
		// mesh triangles are defined relative to their origin so to compare triangle/box
		// we need to transform each triangle by the mesh's pose.
		a, err := boxTriangleIntersectionArea(b, tri.Transform(m.pose))
		if err != nil {
			return -1, err
		}
		totalArea += a
	}
	return totalArea, nil
}

// boxTriangleIntersectionArea calculates the area of intersection between a box and a triangle.
// Returns 0 if there's no intersection, the full triangle area if fully enclosed,
// or the actual intersection area otherwise.
func boxTriangleIntersectionArea(b *box, t *Triangle) (float64, error) {
	// Quick check if they don't intersect at all
	mesh := NewMesh(NewZeroPose(), []*Triangle{t}, "")
	collides, err := b.CollidesWith(mesh, defaultCollisionBufferMM)
	if err != nil {
		return -1, err
	}
	if !collides {
		return 0, nil
	}

	// Check if triangle is fully enclosed by the box
	enclosed := true
	for _, pt := range t.Points() {
		if !pointVsBoxCollision(pt, b, defaultCollisionBufferMM) {
			enclosed = false
			break
		}
	}
	if enclosed {
		return t.Area(), nil
	}

	// Clip triangle against each of the six box planes
	vertices := t.Points()

	// Get box in world space
	boxPose := b.Pose()
	boxCenter := boxPose.Point()
	boxRM := boxPose.Orientation().RotationMatrix()

	// For each of the six box faces, clip the polygon
	for faceIdx := 0; faceIdx < 6; faceIdx++ {
		// Determine face normal and position
		axis := faceIdx / 2                // 0 for X, 1 for Y, 2 for Z
		sign := float64(1 - 2*(faceIdx%2)) // +1 for even indices, -1 for odd indices

		// Get face normal in world coordinates
		normal := boxRM.Row(axis).Mul(sign)

		// Get face point in world coordinates
		facePoint := boxCenter.Add(boxRM.Row(axis).Mul(sign * b.halfSize[axis]))

		// Clip polygon against this plane
		vertices = clipPolygonAgainstPlane(vertices, facePoint, normal)

		// If no vertices left, intersection area is 0
		if len(vertices) < 3 {
			return 0, nil
		}
	}

	// Calculate area of the resulting polygon by triangulating it
	// TODO: all passed in vertices should be coplanar with the triangle normal but this is not explicitly checked
	return calculatePolygonAreaWithTriangulation(vertices), nil
}

// clipPolygonAgainstPlane clips a convex polygon against a plane and returns the vertices of the clipped polygon.
func clipPolygonAgainstPlane(vertices []r3.Vector, planePoint, planeNormal r3.Vector) []r3.Vector {
	if len(vertices) < 3 {
		return vertices
	}

	result := make([]r3.Vector, 0, len(vertices)*2)

	// For each edge in the polygon
	for i := 0; i < len(vertices); i++ {
		j := (i + 1) % len(vertices)

		// Get signed distances from vertices to plane
		di := planeNormal.Dot(vertices[i].Sub(planePoint))
		dj := planeNormal.Dot(vertices[j].Sub(planePoint))

		// If current vertex is inside (negative dot product)
		if di <= floatEpsilon {
			result = append(result, vertices[i])
		}

		// If edge crosses the plane (vertices on opposite sides)
		if (di * dj) < 0 {
			// Calculate intersection point
			t := di / (di - dj)
			intersection := vertices[i].Add(vertices[j].Sub(vertices[i]).Mul(t))
			result = append(result, intersection)
		}
	}

	return result
}

// calculatePolygonAreaWithTriangulation calculates the area of a polygon by triangulating it. All provided vertices must be coplanar.
// TODO: nothing is enforcing that the vertices be coplanar.
func calculatePolygonAreaWithTriangulation(vertices []r3.Vector) float64 {
	// For a malformed polygon there will be no area.
	switch length := len(vertices); {
	case length < 3:
		return 0
	case length == 3:
		// For a 3-vertex polygon, just calculate triangle area directly
		return NewTriangle(vertices[0], vertices[1], vertices[2]).Area()
	default:
		// For polygons with more vertices, triangulate using fan triangulation
		// This works for convex polygons, which is what we have after clipping
		totalArea := 0.0
		for i := 1; i < len(vertices)-1; i++ {
			totalArea += NewTriangle(vertices[0], vertices[i], vertices[i+1]).Area()
		}
		return totalArea
	}
}

// TrianglesToPLYBytes converts the mesh's triangles to bytes in PLY format. The boolean determines
// whether to convert to the world frame or keep it in the local frame.
func (m *Mesh) TrianglesToPLYBytes(convertToWorldFrame bool) []byte {
	// Collect all unique vertices and create vertex-to-index mapping
	vertexMap := make(map[string]int)
	vertices := make([]r3.Vector, 0)

	for _, tri := range m.triangles {
		if convertToWorldFrame {
			tri = tri.Transform(m.pose)
		}
		for _, pt := range tri.Points() {
			scaledPt := r3.Vector{X: pt.X / 1000.0, Y: pt.Y / 1000.0, Z: pt.Z / 1000.0}
			key := fmt.Sprintf("%.10f,%.10f,%.10f", scaledPt.X, scaledPt.Y, scaledPt.Z)
			if _, exists := vertexMap[key]; !exists {
				vertexMap[key] = len(vertices)
				vertices = append(vertices, scaledPt)
			}
		}
	}

	var buf bytes.Buffer

	// Write PLY header
	buf.WriteString("ply\n")
	buf.WriteString("format ascii 1.0\n")
	buf.WriteString(fmt.Sprintf("element vertex %d\n", len(vertices)))
	buf.WriteString("property float x\n")
	buf.WriteString("property float y\n")
	buf.WriteString("property float z\n")
	buf.WriteString(fmt.Sprintf("element face %d\n", len(m.triangles)))
	buf.WriteString("property list uchar int vertex_indices\n")
	buf.WriteString("end_header\n")

	// Write vertices
	for _, vertex := range vertices {
		buf.WriteString(fmt.Sprintf("%f %f %f\n", vertex.X, vertex.Y, vertex.Z))
	}

	// Write faces
	for _, tri := range m.triangles {
		if convertToWorldFrame {
			tri = tri.Transform(m.pose)
		}
		buf.WriteString("3")
		for _, pt := range tri.Points() {
			// Convert from millimeters back to meters for lookup
			scaledPt := r3.Vector{X: pt.X / 1000.0, Y: pt.Y / 1000.0, Z: pt.Z / 1000.0}
			key := fmt.Sprintf("%.10f,%.10f,%.10f", scaledPt.X, scaledPt.Y, scaledPt.Z)
			buf.WriteString(fmt.Sprintf(" %d", vertexMap[key]))
		}
		buf.WriteString("\n")
	}

	return buf.Bytes()
}

// Hash returns a hash value for this mesh.
func (m *Mesh) Hash() int {
	hash := HashPose(m.pose)
	hash += hashString(m.label) * 11
	hash += len(m.triangles) * 12
	// Include a sample of triangle hashes for efficiency
	for i, tri := range m.triangles {
		if i >= 10 { // Only hash first 10 triangles for performance
			break
		}
		hash += tri.Hash() * (13 + i)
	}
	return hash
}
