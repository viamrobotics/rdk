package spatialmath

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"sync"

	"github.com/chenzhekl/goply"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// This file incorporates work covered by the Brax project -- https://github.com/google/brax/blob/main/LICENSE.
// Copyright 2021 The Brax Authors, which is licensed under the Apache License Version 2.0 (the "License").
// You may obtain a copy of the license at http://www.apache.org/licenses/LICENSE-2.0.

// The set of supported mesh file types.
type meshType string

const (
	plyType = meshType("ply")
	stlType = meshType("stl")
)

// Mesh is a set of triangles at some pose. Triangle points are in the frame of the mesh.
type Mesh struct {
	pose      Pose
	triangles []*Triangle
	label     string

	// information used for encoding to protobuf
	fileType meshType
	rawBytes []byte

	// originalFilePath stores the original URDF mesh path for round-tripping
	originalFilePath string

	// bvh is the bounding volume hierarchy for accelerated collision detection.
	// Built lazily on first collision check via ensureBVH().
	bvh     *bvhNode
	bvhOnce sync.Once
}

// trianglesToGeoms converts a slice of triangles to Geometry without transforming them.
// The triangles remain in local space.
func trianglesToGeoms(triangles []*Triangle) []Geometry {
	geoms := make([]Geometry, len(triangles))
	for i, t := range triangles {
		geoms[i] = t
	}
	return geoms
}

// NewMesh creates a mesh from the given triangles and pose.
// The BVH is built lazily on first collision check for faster mesh loading.
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
	mesh, err := newMeshFromBytes(NewZeroPose(), bytes, path)
	if err != nil {
		return nil, err
	}
	mesh.SetOriginalFilePath(path)
	return mesh, nil
}

// NewMeshFromSTLFile is a helper function to create a Mesh geometry from an STL file.
func NewMeshFromSTLFile(path string) (*Mesh, error) {
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
	mesh, err := newMeshFromSTLBytes(NewZeroPose(), bytes, path)
	if err != nil {
		return nil, err
	}
	mesh.SetOriginalFilePath(path)
	return mesh, nil
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

func newMeshFromSTLBytes(pose Pose, data []byte, label string) (*Mesh, error) {
	// Binary STL format:
	// 80 bytes - header
	// 4 bytes - number of triangles (uint32, little endian)
	// For each triangle (50 bytes total):
	//   12 bytes - normal vector (3 floats)
	//   12 bytes - vertex 1 (3 floats)
	//   12 bytes - vertex 2 (3 floats)
	//   12 bytes - vertex 3 (3 floats)
	//   2 bytes - attribute byte count (unused)

	if len(data) < 84 {
		return nil, errors.New("STL file too small")
	}

	// Read number of triangles (bytes 80-83, little endian uint32)
	numTriangles := uint32(data[80]) | uint32(data[81])<<8 | uint32(data[82])<<16 | uint32(data[83])<<24

	expectedSize := 84 + int(numTriangles)*50
	if len(data) < expectedSize {
		return nil, fmt.Errorf("STL file size mismatch: expected %d bytes, got %d", expectedSize, len(data))
	}

	triangles := make([]*Triangle, numTriangles)
	offset := 84

	for i := uint32(0); i < numTriangles; i++ {
		// Skip normal vector (12 bytes)
		offset += 12

		// Read 3 vertices
		v1 := readSTLVertex(data, offset)
		offset += 12
		v2 := readSTLVertex(data, offset)
		offset += 12
		v3 := readSTLVertex(data, offset)
		offset += 12

		// Skip attribute byte count (2 bytes)
		offset += 2

		triangles[i] = NewTriangle(v1, v2, v3)
	}
	return &Mesh{
		pose:      pose,
		triangles: triangles,
		label:     label,
		fileType:  stlType,
		rawBytes:  data,
	}, nil
}

// readSTLVertex reads a 3D vertex from STL binary data at the given offset.
// Each coordinate is a 32-bit float in little endian format.
// Converts from meters to millimeters by multiplying by 1000.
func readSTLVertex(data []byte, offset int) r3.Vector {
	x := math.Float32frombits(uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24)
	y := math.Float32frombits(uint32(data[offset+4]) | uint32(data[offset+5])<<8 | uint32(data[offset+6])<<16 | uint32(data[offset+7])<<24)
	z := math.Float32frombits(uint32(data[offset+8]) | uint32(data[offset+9])<<8 | uint32(data[offset+10])<<16 | uint32(data[offset+11])<<24)

	// Convert from meters to millimeters
	return r3.Vector{X: float64(x) * 1000, Y: float64(y) * 1000, Z: float64(z) * 1000}
}

// NewMeshFromProto creates a new mesh from a protobuf mesh.
func NewMeshFromProto(pose Pose, m *commonpb.Mesh, label string) (*Mesh, error) {
	switch m.ContentType {
	case string(plyType):
		return newMeshFromBytes(pose, m.Mesh, label)
	case string(stlType):
		return newMeshFromSTLBytes(pose, m.Mesh, label)
	default:
		return nil, fmt.Errorf("unsupported Mesh type: %s", m.ContentType)
	}
}

// SetOriginalFilePath sets the original URDF file path for this mesh.
// This is used to preserve the mesh path when round-tripping through URDF.
func (m *Mesh) SetOriginalFilePath(path string) {
	m.originalFilePath = path
}

// OriginalFilePath returns the original URDF file path for this mesh, if set.
func (m *Mesh) OriginalFilePath() string {
	return m.originalFilePath
}

// String returns a human readable string that represents the box.
func (m *Mesh) String() string {
	return fmt.Sprintf("Type: Mesh | Position: X:%.1f, Y:%.1f, Z:%.1f | Triangle count: %d",
		m.pose.Point().X, m.pose.Point().Y, m.pose.Point().Z, len(m.triangles))
}

// ToProtobuf converts a Mesh to its protobuf representation.
// Meshes are always converted to PLY format for compatibility with the visualizer.
// Note that if the mesh's rawBytes and fileType fields are unset this will result in a malformed message
func (m *Mesh) ToProtobuf() *commonpb.Geometry {
	// Convert mesh to PLY format for visualizer compatibility
	// The visualizer expects all meshes to be in PLY format
	plyBytes := m.TrianglesToPLYBytes(false)

	return &commonpb.Geometry{
		Center: PoseToProtobuf(m.pose),
		GeometryType: &commonpb.Geometry_Mesh{
			Mesh: &commonpb.Mesh{
				ContentType: "ply",
				Mesh:        plyBytes,
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
		pose:             Compose(pose, m.pose),
		triangles:        m.triangles,
		label:            m.label,
		fileType:         m.fileType,
		rawBytes:         m.rawBytes,
		originalFilePath: m.originalFilePath,
		bvh:              m.bvh,
	}
}

// CollidesWith checks if the given mesh collides with the given geometry and returns true if it
// does. If there's no collision, the method will return the distance between the mesh and input
// geometry. If there is a collision, a negative number is returned.
func (m *Mesh) CollidesWith(g Geometry, collisionBufferMM float64) (bool, float64, error) {
	switch other := g.(type) {
	case *box:
		// Mesh-ifying the box misses the case where the box encompasses a mesh triangle without its surface intersecting a triangle.
		encompassed := m.boxIntersectsVertex(other)
		if encompassed {
			return true, -1, nil
		}
		return m.collidesWithGeometryBVH(other, collisionBufferMM)
	case *capsule, *point, *sphere, *Mesh:
		return m.collidesWithGeometryBVH(other, collisionBufferMM)
	case *Triangle:
		triMesh := NewMesh(NewZeroPose(), []*Triangle{other}, "")
		return m.collidesWithMesh(triMesh, collisionBufferMM)
	default:
		return true, math.Inf(1), newCollisionTypeUnsupportedError(m, g)
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
		collides, _, err := NewPoint(pt, "").CollidesWith(g, defaultCollisionBufferMM)
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
		return m.distanceFromMesh(other.toMesh())
	case *capsule:
		return capsuleVsMeshDistance(other, m), nil
	case *point:
		return m.distanceFromSphere(&sphere{pose: NewPoseFromPoint(other.position)}), nil
	case *sphere:
		return m.distanceFromSphere(other), nil
	case *Triangle:
		triMesh := NewMesh(NewZeroPose(), []*Triangle{other}, "")
		return m.distanceFromMesh(triMesh)
	case *Mesh:
		return m.distanceFromMesh(other)
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
			c, _ := pointVsBoxCollision(worldPt, b, defaultCollisionBufferMM)
			if c {
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
		closestPt := ClosestPointTrianglePoint(tri.Transform(m.pose).(*Triangle), pt)
		dist := closestPt.Sub(pt).Norm() - s.radius
		if dist < minDist {
			minDist = dist
		}
	}
	return minDist
}

// ensureBVH builds the BVH if it hasn't been built yet (thread-safe).
// Returns nil for empty meshes (no triangles).
func (m *Mesh) ensureBVH() *bvhNode {
	m.bvhOnce.Do(func() {
		// Check if bvh was already set (e.g., copied from another mesh via Transform)
		if m.bvh == nil && len(m.triangles) > 0 {
			m.bvh = buildBVH(trianglesToGeoms(m.triangles))
		}
	})
	return m.bvh
}

// collidesWithMesh checks if this mesh collides with another mesh.
// Uses BVH acceleration for O(log n * log m) performance instead of O(n*m).
func (m *Mesh) collidesWithMesh(other *Mesh, collisionBufferMM float64) (bool, float64, error) {
	if len(m.triangles) == 0 || len(other.triangles) == 0 {
		return false, 0, errors.New("cannot check collision on mesh with no triangles")
	}
	// Pass poses to BVH collision - BVH stores geometries in local space
	return bvhCollidesWithBVH(m.ensureBVH(), other.ensureBVH(), m.pose, other.pose, collisionBufferMM)
}

// collidesWithGeometryBVH uses BVH to accelerate mesh vs single geometry collision.
func (m *Mesh) collidesWithGeometryBVH(other Geometry, collisionBufferMM float64) (bool, float64, error) {
	bvh := m.ensureBVH()
	if bvh == nil {
		return false, 0, errors.New("cannot check collision on mesh with no triangles")
	}
	otherMin, otherMax := computeGeometryAABB(other)
	// Pass mesh pose to BVH collision - BVH stores geometries in local space
	return bvhCollidesWithGeometry(bvh, m.pose, other, otherMin, otherMax, collisionBufferMM)
}

// distanceFromMesh returns the minimum distance between this mesh and another mesh.
// Uses BVH acceleration for O(log n * log m) performance.
func (m *Mesh) distanceFromMesh(other *Mesh) (float64, error) {
	if len(m.triangles) == 0 || len(other.triangles) == 0 {
		return 0, errors.New("cannot compute distance on mesh with no triangles")
	}
	// Pass poses to BVH distance - BVH stores geometries in local space
	return bvhDistanceFromBVH(m.ensureBVH(), other.ensureBVH(), m.pose, other.pose)
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
	// Copy fields explicitly to avoid copying sync.Once.
	m.pose = mesh.pose
	m.triangles = mesh.triangles
	m.label = mesh.label
	m.fileType = mesh.fileType
	m.rawBytes = mesh.rawBytes
	m.originalFilePath = mesh.originalFilePath
	m.bvh = nil
	m.bvhOnce = sync.Once{}
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (m *Mesh) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(m.ToProtobuf())
}

// TrianglesToPLYBytes converts the mesh's triangles to bytes in PLY format. The boolean determines
// whether to convert to the world frame or keep it in the local frame.
func (m *Mesh) TrianglesToPLYBytes(convertToWorldFrame bool) []byte {
	// Collect all unique vertices and create vertex-to-index mapping
	vertexMap := make(map[string]int)
	vertices := make([]r3.Vector, 0)

	for _, tri := range m.triangles {
		if convertToWorldFrame {
			tri = tri.Transform(m.pose).(*Triangle)
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
			tri = tri.Transform(m.pose).(*Triangle)
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
