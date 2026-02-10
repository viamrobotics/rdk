package spatialmath

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
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

	// DefaultConservativeDecimatedTriangleCount is the default target triangle count used by
	// conservative mesh simplification for collision checks.
	DefaultConservativeDecimatedTriangleCount = 2000
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

	mesh = maybeAutoConservativeDecimateMesh(mesh)

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
	mesh = maybeAutoConservativeDecimateMesh(mesh)
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
	mesh = maybeAutoConservativeDecimateMesh(mesh)
	mesh.SetOriginalFilePath(path)
	return mesh, nil
}

func maybeAutoConservativeDecimateMesh(m *Mesh) *Mesh {
	if len(m.triangles) <= DefaultConservativeDecimatedTriangleCount {
		return m
	}
	decimated, err := m.ConservativeDecimateToDefault()
	if err != nil {
		// Fallback to the original mesh if simplification fails.
		return m
	}
	return decimated
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
	mesh = &Mesh{
		pose:      pose,
		triangles: triangles,
		label:     label,
		fileType:  plyType,
		rawBytes:  data,
	}
	return maybeAutoConservativeDecimateMesh(mesh), nil
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
	mesh := &Mesh{
		pose:      pose,
		triangles: triangles,
		label:     label,
		fileType:  stlType,
		rawBytes:  data,
	}
	return maybeAutoConservativeDecimateMesh(mesh), nil
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
	var mesh *Mesh
	var err error
	switch m.ContentType {
	case string(plyType):
		mesh, err = newMeshFromBytes(pose, m.Mesh, label)
	case string(stlType):
		mesh, err = newMeshFromSTLBytes(pose, m.Mesh, label)
	default:
		return nil, fmt.Errorf("unsupported Mesh type: %s", m.ContentType)
	}
	if err != nil {
		return nil, err
	}
	return maybeAutoConservativeDecimateMesh(mesh), nil
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

// ConservativeDecimate returns a mesh with at most the requested number of triangles.
// If this mesh has more triangles than requested, it is replaced by an enclosing conservative hull mesh
// that guarantees containment and avoids collision false negatives.
func (m *Mesh) ConservativeDecimate(targetTriangles int) (*Mesh, error) {
	if targetTriangles <= 0 {
		return nil, errors.New("target triangle count must be positive")
	}
	if len(m.triangles) == 0 {
		return nil, errors.New("cannot decimate mesh with no triangles")
	}
	if targetTriangles < len(boxTriangles) {
		return nil, errors.Errorf("target triangle count must be at least %d", len(boxTriangles))
	}
	if len(m.triangles) <= targetTriangles {
		return m, nil
	}

	enclosingTris, err := conservativeHullDecimateTriangles(m.triangles, targetTriangles)
	if err != nil {
		// Fallback for degenerate/pathological meshes.
		minPt, maxPt := localAABBForTriangles(m.triangles)
		enclosingTris = tessellatedAABBTriangles(minPt, maxPt, targetTriangles)
	}

	decimated := &Mesh{
		pose:      m.pose,
		triangles: enclosingTris,
		label:     m.label,
		fileType:  plyType,
	}
	decimated.rawBytes = decimated.TrianglesToPLYBytes(false)
	decimated.SetOriginalFilePath(m.originalFilePath)
	return decimated, nil
}

// ConservativeDecimateToDefault decimates meshes larger than 2000 triangles to <= 2000 triangles.
func (m *Mesh) ConservativeDecimateToDefault() (*Mesh, error) {
	return m.ConservativeDecimate(DefaultConservativeDecimatedTriangleCount)
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
	switch other := g.(type) {
	case *point:
		return false, nil
	case *Mesh:
		// Meshes are not treated as solid volumes for collision checks, so this uses conservative
		// AABB containment to support collision-safe simplification checks.
		return m.encompassedByMeshAABB(other), nil
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

func (m *Mesh) encompassedByMeshAABB(other *Mesh) bool {
	if len(other.triangles) == 0 {
		return false
	}
	minPt, maxPt := computeMeshAABB(other)
	eps := defaultCollisionBufferMM
	for _, pt := range m.ToPoints(1) {
		if pt.X < minPt.X-eps || pt.X > maxPt.X+eps ||
			pt.Y < minPt.Y-eps || pt.Y > maxPt.Y+eps ||
			pt.Z < minPt.Z-eps || pt.Z > maxPt.Z+eps {
			return false
		}
	}
	return true
}

type quickHullFace struct {
	a, b, c int
	normal  r3.Vector
	offset  float64
	outside []int
	deleted bool
}

func conservativeHullDecimateTriangles(triangles []*Triangle, targetTriangles int) ([]*Triangle, error) {
	if targetTriangles < len(boxTriangles) {
		return nil, errors.Errorf("target triangle count must be at least %d", len(boxTriangles))
	}

	vertices := uniqueTriangleVertices(triangles)
	if len(vertices) < 4 {
		return nil, errors.New("need at least 4 unique vertices to build conservative hull")
	}

	// For triangular convex hulls, F <= 2V-4. Keep V bounded so F stays <= target.
	vertexBudget := (targetTriangles + 4) / 2
	if vertexBudget < 4 {
		vertexBudget = 4
	}

	hullInput := vertices
	if len(hullInput) > vertexBudget {
		hullInput = selectSupportVertices(vertices, vertexBudget)
	}

	faces, hullPoints, err := quickHull3D(hullInput, floatEpsilon)
	if err != nil {
		return nil, err
	}
	hullTris := hullFacesToTriangles(faces, hullPoints)
	if len(hullTris) == 0 {
		return nil, errors.New("failed to build conservative hull")
	}

	// Strict containment: if sampled hull misses extremes, scale it outward just enough to contain all vertices.
	hullCenter := centroidOfPoints(hullPoints)
	scale := requiredHullScale(vertices, faces, hullCenter)
	if scale > 1.0 {
		hullTris = scaleTrianglesAboutPoint(hullTris, hullCenter, scale*(1.0+1e-9))
	}

	if len(hullTris) > targetTriangles {
		return nil, errors.Errorf("conservative hull has %d triangles, expected <= %d", len(hullTris), targetTriangles)
	}
	return hullTris, nil
}

func uniqueTriangleVertices(triangles []*Triangle) []r3.Vector {
	pointMap := make(map[string]r3.Vector)
	for _, tri := range triangles {
		for _, pt := range tri.Points() {
			key := fmt.Sprintf("%.10f,%.10f,%.10f", pt.X, pt.Y, pt.Z)
			pointMap[key] = pt
		}
	}
	out := make([]r3.Vector, 0, len(pointMap))
	for _, pt := range pointMap {
		out = append(out, pt)
	}
	return out
}

func selectSupportVertices(vertices []r3.Vector, maxPoints int) []r3.Vector {
	if len(vertices) <= maxPoints {
		out := make([]r3.Vector, len(vertices))
		copy(out, vertices)
		return out
	}

	directions := fibonacciSphereDirections(maxPoints)
	directions = append(directions,
		r3.Vector{X: 1, Y: 0, Z: 0}, r3.Vector{X: -1, Y: 0, Z: 0},
		r3.Vector{X: 0, Y: 1, Z: 0}, r3.Vector{X: 0, Y: -1, Z: 0},
		r3.Vector{X: 0, Y: 0, Z: 1}, r3.Vector{X: 0, Y: 0, Z: -1},
	)

	supportMap := make(map[string]r3.Vector)
	for _, dir := range directions {
		best := vertices[0]
		bestDot := best.Dot(dir)
		for i := 1; i < len(vertices); i++ {
			d := vertices[i].Dot(dir)
			if d > bestDot {
				bestDot = d
				best = vertices[i]
			}
		}
		key := fmt.Sprintf("%.10f,%.10f,%.10f", best.X, best.Y, best.Z)
		supportMap[key] = best
	}

	support := make([]r3.Vector, 0, len(supportMap))
	for _, pt := range supportMap {
		support = append(support, pt)
	}
	if len(support) > maxPoints {
		center := centroidOfPoints(vertices)
		sort.Slice(support, func(i, j int) bool {
			return support[i].Sub(center).Norm2() > support[j].Sub(center).Norm2()
		})
		support = support[:maxPoints]
	}

	if len(support) < 4 {
		for _, pt := range vertices {
			key := fmt.Sprintf("%.10f,%.10f,%.10f", pt.X, pt.Y, pt.Z)
			if _, ok := supportMap[key]; ok {
				continue
			}
			support = append(support, pt)
			supportMap[key] = pt
			if len(support) >= 4 {
				break
			}
		}
	}
	return support
}

func fibonacciSphereDirections(n int) []r3.Vector {
	if n <= 0 {
		return nil
	}
	if n == 1 {
		return []r3.Vector{{X: 0, Y: 0, Z: 1}}
	}

	goldenAngle := math.Pi * (3 - math.Sqrt(5))
	dirs := make([]r3.Vector, n)
	for i := range n {
		y := 1 - (2 * float64(i) / float64(n-1))
		radius := math.Sqrt(math.Max(0, 1-y*y))
		theta := goldenAngle * float64(i)
		dirs[i] = r3.Vector{
			X: math.Cos(theta) * radius,
			Y: y,
			Z: math.Sin(theta) * radius,
		}
	}
	return dirs
}

func quickHull3D(points []r3.Vector, eps float64) ([]quickHullFace, []r3.Vector, error) {
	if len(points) < 4 {
		return nil, nil, errors.New("need at least 4 points for 3D hull")
	}

	i0, i1 := 0, 0
	for i := 1; i < len(points); i++ {
		if points[i].X < points[i0].X {
			i0 = i
		}
		if points[i].X > points[i1].X {
			i1 = i
		}
	}
	if i0 == i1 {
		return nil, nil, errors.New("degenerate point set")
	}

	lineDir := points[i1].Sub(points[i0])
	i2, maxLineDist := -1, -1.0
	for i := range points {
		if i == i0 || i == i1 {
			continue
		}
		dist := lineDir.Cross(points[i].Sub(points[i0])).Norm()
		if dist > maxLineDist {
			maxLineDist = dist
			i2 = i
		}
	}
	if i2 < 0 || maxLineDist <= eps {
		return nil, nil, errors.New("points are nearly collinear")
	}

	baseNormal := PlaneNormal(points[i0], points[i1], points[i2])
	i3, maxPlaneDist := -1, -1.0
	for i := range points {
		if i == i0 || i == i1 || i == i2 {
			continue
		}
		dist := math.Abs(baseNormal.Dot(points[i].Sub(points[i0])))
		if dist > maxPlaneDist {
			maxPlaneDist = dist
			i3 = i
		}
	}
	if i3 < 0 || maxPlaneDist <= eps {
		return nil, nil, errors.New("points are nearly coplanar")
	}

	interior := centroidOfPoints([]r3.Vector{points[i0], points[i1], points[i2], points[i3]})
	faces := []quickHullFace{
		newQuickHullFace(points, i0, i1, i2, interior),
		newQuickHullFace(points, i0, i3, i1, interior),
		newQuickHullFace(points, i1, i3, i2, interior),
		newQuickHullFace(points, i2, i3, i0, interior),
	}

	tetra := map[int]struct{}{i0: {}, i1: {}, i2: {}, i3: {}}
	for pIdx := range points {
		if _, ok := tetra[pIdx]; ok {
			continue
		}
		assignPointToHullFace(points, pIdx, faces, eps)
	}

	for {
		faceIdx := -1
		for i := range faces {
			if !faces[i].deleted && len(faces[i].outside) > 0 {
				faceIdx = i
				break
			}
		}
		if faceIdx < 0 {
			break
		}

		eye := farthestOutsidePoint(points, faces[faceIdx])
		if eye < 0 {
			faces[faceIdx].outside = nil
			continue
		}

		visible := make([]int, 0)
		for i := range faces {
			if faces[i].deleted {
				continue
			}
			if facePointDistance(faces[i], points[eye]) > eps {
				visible = append(visible, i)
			}
		}
		if len(visible) == 0 {
			faces[faceIdx].outside = removePointFromSlice(faces[faceIdx].outside, eye)
			continue
		}

		horizon := make(map[[2]int]struct{})
		reassign := make(map[int]struct{})
		for _, vi := range visible {
			f := &faces[vi]
			for _, pIdx := range f.outside {
				if pIdx != eye {
					reassign[pIdx] = struct{}{}
				}
			}
			f.deleted = true
			addHorizonEdge(horizon, f.a, f.b)
			addHorizonEdge(horizon, f.b, f.c)
			addHorizonEdge(horizon, f.c, f.a)
		}
		if len(horizon) == 0 {
			continue
		}

		newFaces := make([]int, 0, len(horizon))
		for edge := range horizon {
			nf := newQuickHullFace(points, edge[0], edge[1], eye, interior)
			if nf.normal.Norm2() <= 0 {
				continue
			}
			faces = append(faces, nf)
			newFaces = append(newFaces, len(faces)-1)
		}
		if len(newFaces) == 0 {
			continue
		}

		for pIdx := range reassign {
			bestFace, bestDist := -1, eps
			for _, fi := range newFaces {
				d := facePointDistance(faces[fi], points[pIdx])
				if d > bestDist {
					bestDist = d
					bestFace = fi
				}
			}
			if bestFace >= 0 {
				faces[bestFace].outside = append(faces[bestFace].outside, pIdx)
			}
		}
	}

	return faces, points, nil
}

func newQuickHullFace(points []r3.Vector, a, b, c int, interior r3.Vector) quickHullFace {
	normal := PlaneNormal(points[a], points[b], points[c])
	if normal.Norm2() <= 0 {
		return quickHullFace{a: a, b: b, c: c}
	}
	offset := normal.Dot(points[a])
	if normal.Dot(interior)-offset > 0 {
		b, c = c, b
		normal = PlaneNormal(points[a], points[b], points[c])
		offset = normal.Dot(points[a])
	}
	return quickHullFace{
		a:      a,
		b:      b,
		c:      c,
		normal: normal,
		offset: offset,
	}
}

func facePointDistance(face quickHullFace, pt r3.Vector) float64 {
	return face.normal.Dot(pt) - face.offset
}

func assignPointToHullFace(points []r3.Vector, pIdx int, faces []quickHullFace, eps float64) {
	bestFace, bestDist := -1, eps
	for i := range faces {
		if faces[i].deleted {
			continue
		}
		d := facePointDistance(faces[i], points[pIdx])
		if d > bestDist {
			bestDist = d
			bestFace = i
		}
	}
	if bestFace >= 0 {
		faces[bestFace].outside = append(faces[bestFace].outside, pIdx)
	}
}

func farthestOutsidePoint(points []r3.Vector, face quickHullFace) int {
	bestIdx := -1
	bestDist := -1.0
	for _, pIdx := range face.outside {
		d := facePointDistance(face, points[pIdx])
		if d > bestDist {
			bestDist = d
			bestIdx = pIdx
		}
	}
	return bestIdx
}

func addHorizonEdge(horizon map[[2]int]struct{}, a, b int) {
	rev := [2]int{b, a}
	if _, ok := horizon[rev]; ok {
		delete(horizon, rev)
		return
	}
	horizon[[2]int{a, b}] = struct{}{}
}

func removePointFromSlice(points []int, target int) []int {
	out := points[:0]
	for _, p := range points {
		if p != target {
			out = append(out, p)
		}
	}
	return out
}

func hullFacesToTriangles(faces []quickHullFace, points []r3.Vector) []*Triangle {
	tris := make([]*Triangle, 0, len(faces))
	for _, face := range faces {
		if face.deleted || face.normal.Norm2() <= 0 {
			continue
		}
		tris = append(tris, NewTriangle(points[face.a], points[face.b], points[face.c]))
	}
	return tris
}

func centroidOfPoints(points []r3.Vector) r3.Vector {
	if len(points) == 0 {
		return r3.Vector{}
	}
	acc := r3.Vector{}
	for _, p := range points {
		acc = acc.Add(p)
	}
	return acc.Mul(1.0 / float64(len(points)))
}

func requiredHullScale(original []r3.Vector, faces []quickHullFace, center r3.Vector) float64 {
	scale := 1.0
	for _, face := range faces {
		if face.deleted || face.normal.Norm2() <= 0 {
			continue
		}
		centerDot := face.normal.Dot(center)
		denom := face.offset - centerDot
		if denom <= floatEpsilon {
			continue
		}
		for _, pt := range original {
			num := face.normal.Dot(pt) - centerDot
			required := (num + defaultCollisionBufferMM) / denom
			if required > scale {
				scale = required
			}
		}
	}
	return scale
}

func scaleTrianglesAboutPoint(triangles []*Triangle, center r3.Vector, scale float64) []*Triangle {
	scaled := make([]*Triangle, len(triangles))
	for i, tri := range triangles {
		pts := tri.Points()
		scaled[i] = NewTriangle(
			center.Add(pts[0].Sub(center).Mul(scale)),
			center.Add(pts[1].Sub(center).Mul(scale)),
			center.Add(pts[2].Sub(center).Mul(scale)),
		)
	}
	return scaled
}

func localAABBForTriangles(triangles []*Triangle) (r3.Vector, r3.Vector) {
	minPt := r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	maxPt := r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
	for _, tri := range triangles {
		for _, pt := range tri.Points() {
			minPt, maxPt = expandAABB(minPt, maxPt, pt)
		}
	}
	return minPt, maxPt
}

func tessellatedAABBTriangles(minPt, maxPt r3.Vector, targetTriangles int) []*Triangle {
	triangles := meshTrianglesForAABB(minPt, maxPt)
	for len(triangles) < targetTriangles {
		idx := largestTriangleIndex(triangles)
		t0, t1 := splitTriangleOnLongestEdge(triangles[idx])
		triangles[idx] = t0
		triangles = append(triangles, t1)
	}
	return triangles
}

func meshTrianglesForAABB(minPt, maxPt r3.Vector) []*Triangle {
	center := minPt.Add(maxPt).Mul(0.5)
	half := maxPt.Sub(minPt).Mul(0.5)

	verts := make([]r3.Vector, len(boxVertices))
	for i, v := range boxVertices {
		verts[i] = r3.Vector{
			X: center.X + v.X*half.X,
			Y: center.Y + v.Y*half.Y,
			Z: center.Z + v.Z*half.Z,
		}
	}

	triangles := make([]*Triangle, 0, len(boxTriangles))
	for _, tri := range boxTriangles {
		triangles = append(triangles, NewTriangle(verts[tri[0]], verts[tri[1]], verts[tri[2]]))
	}
	return triangles
}

func largestTriangleIndex(triangles []*Triangle) int {
	largestIdx := 0
	largestArea := triangles[0].Area()
	for i := 1; i < len(triangles); i++ {
		area := triangles[i].Area()
		if area > largestArea {
			largestArea = area
			largestIdx = i
		}
	}
	return largestIdx
}

func splitTriangleOnLongestEdge(tri *Triangle) (*Triangle, *Triangle) {
	pts := tri.Points()
	edgeA, edgeB, opposite := 0, 1, 2
	longest := pts[0].Sub(pts[1]).Norm2()

	if edgeLen := pts[1].Sub(pts[2]).Norm2(); edgeLen > longest {
		edgeA, edgeB, opposite = 1, 2, 0
		longest = edgeLen
	}
	if edgeLen := pts[2].Sub(pts[0]).Norm2(); edgeLen > longest {
		edgeA, edgeB, opposite = 2, 0, 1
	}

	mid := pts[edgeA].Add(pts[edgeB]).Mul(0.5)
	return NewTriangle(pts[edgeA], mid, pts[opposite]), NewTriangle(mid, pts[edgeB], pts[opposite])
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
