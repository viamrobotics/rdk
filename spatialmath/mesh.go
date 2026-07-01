package spatialmath

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"sync/atomic"

	"github.com/chenzhekl/goply"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	commonpb "go.viam.com/api/common/v1"
	"gonum.org/v1/gonum/num/quat"
	"google.golang.org/protobuf/encoding/protojson"
)

// meshStateIDCounter assigns a unique uint64 to every meshState at construction
// so the negative cache key can mix in stable pair identity without unsafe.Pointer.
var meshStateIDCounter atomic.Uint64

// newMeshState constructs a meshState with a unique id.
func newMeshState() *meshState {
	return &meshState{id: meshStateIDCounter.Add(1)}
}

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

	// information used for encoding to protobuf. For meshes constructed from
	// raw PLY/STL bytes these are populated up front; for meshes constructed
	// from triangles via NewMesh, PLY bytes are generated lazily on first
	// access via ensurePLYBytes(). The serialization is ASCII PLY and showed
	// up at ~13% of total CPU when eagerly computed inside the collision-check
	// hot path (Mesh.CollidesWith wraps single triangles in throwaway meshes).
	fileType     meshType
	rawBytes     []byte
	rawBytesOnce sync.Once

	// originalFilePath stores the original URDF mesh path for round-tripping
	originalFilePath string

	// bvh is the bounding volume hierarchy for accelerated collision detection.
	// Built lazily on first collision check via ensureBVH().
	bvh     *bvhNode
	bvhOnce sync.Once

	// uniqueVerts is a deduplicated list of triangle vertices in local space.
	// Built lazily via ensureUniqueVertices(). Shared across Transform() copies.
	uniqueVerts     []r3.Vector
	uniqueVertsOnce sync.Once

	// state carries per-logical-mesh witness caches that survive Transform copies
	// (the pointer is shared across Transform-derived Meshes). The hot collision
	// path reaches the witness caches via direct field access; this turned out
	// measurably faster than threading an external cache through the call chain.
	// Planning-level concerns (edge memoization, geometry-pair hints) live one
	// layer up in motionplan.CollisionCache.
	state *meshState
}

// meshState holds the per-logical-mesh witness caches. Its pointer doubles as
// the stable identity used for mesh-vs-mesh witness keying — two Mesh values
// that share a *meshState are the "same logical mesh" for caching purposes.
type meshState struct {
	// id is a process-unique identifier assigned at construction. Used as a
	// stable pair identity in the negative cache hash without resorting to
	// unsafe.Pointer arithmetic.
	id uint64

	// witnesses caches the colliding triangle pair for mesh-vs-mesh queries.
	// Keyed by the other mesh's *meshState pointer; pointer keys avoid the
	// per-call boxing cost of [2]string keys in sync.Map.
	witnesses sync.Map // *meshState -> *witnessPair

	// geomWitness caches the colliding triangle for mesh-vs-non-mesh queries
	// (capsule, box, sphere). Keyed by the other geometry's label — non-mesh
	// geometries don't have a shared meshState pointer to use as identity.
	geomWitness sync.Map // string (other.Label()) -> *Triangle

	// negCache memoizes "no collision" verdicts for mesh-vs-mesh queries.
	// Witness caching only short-circuits *colliding* pairs; in workloads where
	// the same two meshes are repeatedly checked at the same world poses and
	// return "no collision" each time (e.g., arm self-collision between non-
	// adjacent links during RRT smoothing), every call walks the full BVH.
	// Diagnostic on salad1.json showed 7 unique relative poses accounting for
	// ~380 ~1ms BVH walks; caching the verdict turns subsequent calls into a
	// hash lookup.
	//
	// Key is a uint64 hash of (other's *meshState, m.pose, other.pose). Value
	// is *negCacheEntry which carries the original key components so we can
	// verify on lookup — hash collisions are rare but treated as cache misses
	// (the cached entry is overwritten by the next BVH walk anyway).
	negCache sync.Map // uint64 -> *negCacheEntry
}

// witnessPair records a previously-colliding triangle pair so subsequent
// collision queries between the same meshes can short-circuit the BVH walk.
type witnessPair struct {
	t1, t2 *Triangle
}

// negCacheEntry stores a verified "no collision" verdict for a specific pair
// at specific world poses. The pose snapshot lets us reject hash collisions
// before returning a stale result.
type negCacheEntry struct {
	other     *meshState
	aPt, bPt  r3.Vector
	aq, bq    quat.Number
	minDistSq float64
}

// negCacheKey hashes (other identity, both world poses) into a uint64. FNV-1a
// over the 14 pose floats plus the other-state pointer address. We then verify
// the actual values on lookup to neutralize hash collisions.
func negCacheKey(other *meshState, aPose, bPose Pose) uint64 {
	apt := aPose.Point()
	bpt := bPose.Point()
	aq := aPose.Orientation().Quaternion()
	bq := bPose.Orientation().Quaternion()
	const fnvPrime = 0x100000001b3
	h := uint64(0xcbf29ce484222325)
	h ^= other.id
	h *= fnvPrime
	for _, v := range [14]float64{
		apt.X, apt.Y, apt.Z, aq.Real, aq.Imag, aq.Jmag, aq.Kmag,
		bpt.X, bpt.Y, bpt.Z, bq.Real, bq.Imag, bq.Jmag, bq.Kmag,
	} {
		h ^= math.Float64bits(v)
		h *= fnvPrime
	}
	return h
}

// negCacheEntryMatches verifies that a cache hit really represents the same
// pair-at-same-poses we're querying — guards against the (rare) hash collision.
func negCacheEntryMatches(e *negCacheEntry, other *meshState, aPose, bPose Pose) bool {
	if e.other != other {
		return false
	}
	apt := aPose.Point()
	bpt := bPose.Point()
	if e.aPt != apt || e.bPt != bpt {
		return false
	}
	aq := aPose.Orientation().Quaternion()
	bq := bPose.Orientation().Quaternion()
	return e.aq == aq && e.bq == bq
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
// The BVH and PLY-bytes serialization are both built lazily on first access
// — eager PLY generation was ~13% of total CPU under the planner because
// Mesh.CollidesWith wraps a single triangle in a throwaway Mesh on every call.
func NewMesh(pose Pose, triangles []*Triangle, label string) *Mesh {
	return &Mesh{
		pose:      pose,
		triangles: triangles,
		label:     label,
		state:     newMeshState(),
		fileType:  plyType,
	}
}

// ensurePLYBytes returns the mesh's PLY serialization, generating it on first
// access for triangle-constructed meshes. Meshes loaded from raw PLY/STL bytes
// already have rawBytes populated; the closure then no-ops.
func (m *Mesh) ensurePLYBytes() []byte {
	m.rawBytesOnce.Do(func() {
		if m.rawBytes == nil {
			m.rawBytes = m.TrianglesToPLYBytes(false)
			if m.fileType == "" {
				m.fileType = plyType
			}
		}
	})
	return m.rawBytes
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
		state:     newMeshState(),
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
		state:     newMeshState(),
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
	return &commonpb.Geometry{
		Center: PoseToProtobuf(m.pose),
		GeometryType: &commonpb.Geometry_Mesh{
			Mesh: &commonpb.Mesh{
				ContentType: "ply",
				Mesh:        m.ensurePLYBytes(),
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
		bvh:              m.ensureBVH(),
		uniqueVerts:      m.ensureUniqueVertices(),
		// Share state across transforms so the witness cache survives per-config clones.
		state: m.state,
	}
}

// CollidesWith checks if the given mesh collides with the given geometry and returns true if it
// does. If there's no collision, the method will return the distance between the mesh and input
// geometry. If there is a collision, a negative number is returned.
//
// Mesh-vs-mesh and mesh-vs-non-mesh paths consult the per-mesh witness caches
// on m.state to short-circuit the BVH walk under temporal coherence.
func (m *Mesh) CollidesWith(g Geometry, collisionBufferMM float64) (bool, float64, error) {
	// Witness fast-path for non-mesh geometries. Runs ahead of the box-vertex
	// sweep and the BVH walk so a still-colliding cached triangle short-circuits
	// the whole call.
	if _, isMesh := g.(*Mesh); !isMesh && m.state != nil {
		if label := g.Label(); label != "" {
			if v, ok := m.state.geomWitness.Load(label); ok {
				if witnessTriCollidesWith(v.(*Triangle), m.pose, g, collisionBufferMM) {
					return true, -1, nil
				}
			}
		}
	}

	switch other := g.(type) {
	case *box:
		// Mesh-ifying the box misses the case where the box encompasses a mesh triangle without its surface intersecting a triangle.
		encompassed := m.boxIntersectsVertex(other)
		if encompassed {
			return true, -1, nil
		}
		return m.collidesWithGeometryBVH(other, collisionBufferMM)
	case *Mesh:
		return m.collidesWithMesh(other, collisionBufferMM)
	case *Cylinder:
		return other.CollidesWith(m, collisionBufferMM)
	case *capsule, *point, *sphere:
		return m.collidesWithGeometryBVH(other, collisionBufferMM)
	case *Triangle:
		// Wrap in a Mesh so we get the negative-cache short-circuit in
		// collidesWithMesh — RRT smoothing re-checks the same triangle at the
		// same pose, and the geometry-BVH path has no negCache. The wrap is
		// cheap now that NewMesh defers PLY serialization (ensurePLYBytes).
		triMesh := NewMesh(NewZeroPose(), []*Triangle{other}, "")
		return m.collidesWithMesh(triMesh, collisionBufferMM)
	default:
		return true, math.Inf(1), errCollisionTypeUnsupported
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
	case *Cylinder:
		// Cylinder is convex and Mesh sample points are world-frame; analytic point-in-cylinder
		// avoids the surface-only semantics of Point.CollidesWith(Cylinder).
		for _, pt := range m.ToPoints(1) {
			if !other.containsPoint(pt) {
				return false, nil
			}
		}
		return true, nil
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
	case *Cylinder:
		return other.DistanceFrom(m)
	default:
		return math.Inf(-1), errCollisionTypeUnsupported
	}
}

// ensureUniqueVertices lazily computes the deduplicated vertex list.
func (m *Mesh) ensureUniqueVertices() []r3.Vector {
	m.uniqueVertsOnce.Do(func() {
		if m.uniqueVerts == nil && len(m.triangles) > 0 {
			seen := make(map[r3.Vector]struct{})
			verts := []r3.Vector{}
			for _, tri := range m.triangles {
				for _, pt := range tri.Points() {
					if _, ok := seen[pt]; ok {
						continue
					}
					seen[pt] = struct{}{}
					verts = append(verts, pt)
				}
			}
			m.uniqueVerts = verts
		}
	})
	return m.uniqueVerts
}

// Returns true if any triangle vertex of the mesh intersects the box.
func (m *Mesh) boxIntersectsVertex(b *box) bool {
	q := m.pose.Orientation().Quaternion()
	t := m.pose.Point()
	for _, pt := range m.ensureUniqueVertices() {
		worldPt := TransformPoint(q, t, pt)
		c, _ := pointVsBoxCollision(worldPt, b, defaultCollisionBufferMM)
		if c {
			return true
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

// encompassedByMeshAABB checks whether all vertices of m fall within the world-space AABB of other.
// This is intentionally conservative (loose): a mesh inside the AABB might not be inside the actual
// hull, but a mesh outside the AABB is definitely not inside. This is used only for collision-safe
// simplification validation (e.g. verifying that a decimated hull still encloses the original mesh).
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
//
// Two-tier cache lives on m.state and is shared across all Transform-derived
// copies of the same logical mesh:
//
//   - Positive witness: the previously-colliding triangle pair (keyed by the
//     other mesh's *meshState pointer) is re-checked first; a still-colliding
//     witness short-circuits the BVH walk.
//   - Negative cache: previously-verified "no collision" verdicts keyed by
//     (other-state, both world poses). When the planner re-checks the same
//     pair at the same configuration (common with RRT-Connect rewire and path
//     smoothing) this turns a ~1ms BVH walk into a hash lookup.
func (m *Mesh) collidesWithMesh(other *Mesh, collisionBufferMM float64) (bool, float64, error) {
	if len(m.triangles) == 0 || len(other.triangles) == 0 {
		return false, 0, errors.New("cannot check collision on mesh with no triangles")
	}

	var (
		negKey         uint64
		negKeyComputed bool
	)
	if m.state != nil && other.state != nil {
		if v, ok := m.state.witnesses.Load(other.state); ok {
			wp := v.(*witnessPair)
			if witnessStillCollides(wp.t1, wp.t2, m.pose, other.pose, collisionBufferMM) {
				return true, -1, nil
			}
		}
		negKey = negCacheKey(other.state, m.pose, other.pose)
		negKeyComputed = true
		if v, ok := m.state.negCache.Load(negKey); ok {
			e := v.(*negCacheEntry)
			if negCacheEntryMatches(e, other.state, m.pose, other.pose) {
				return false, math.Sqrt(e.minDistSq), nil
			}
		}
	}

	collides, dist, witness, err := bvhCollidesWithBVHTracked(m.ensureBVH(), other.ensureBVH(), m.pose, other.pose, collisionBufferMM)
	if err != nil {
		return false, 0, err
	}
	if m.state != nil && other.state != nil {
		if collides && witness[0] != nil && witness[1] != nil {
			m.state.witnesses.Store(other.state, &witnessPair{t1: witness[0], t2: witness[1]})
		} else if !collides {
			if !negKeyComputed {
				negKey = negCacheKey(other.state, m.pose, other.pose)
			}
			e := &negCacheEntry{
				other:     other.state,
				aPt:       m.pose.Point(),
				bPt:       other.pose.Point(),
				aq:        m.pose.Orientation().Quaternion(),
				bq:        other.pose.Orientation().Quaternion(),
				minDistSq: dist * dist,
			}
			m.state.negCache.Store(negKey, e)
		}
	}
	return collides, dist, nil
}

// witnessStillCollides re-checks a cached colliding triangle pair under fresh
// poses. Operates on stack-local Triangle values so the cache check itself
// allocates nothing.
func witnessStillCollides(t1, t2 *Triangle, pose1, pose2 Pose, collisionBufferMM float64) bool {
	q1 := pose1.Orientation().Quaternion()
	tr1 := pose1.Point()
	q2 := pose2.Orientation().Quaternion()
	tr2 := pose2.Point()
	worldT1 := Triangle{
		p0:     TransformPoint(q1, tr1, t1.p0),
		p1:     TransformPoint(q1, tr1, t1.p1),
		p2:     TransformPoint(q1, tr1, t1.p2),
		normal: transformDirection(q1, t1.normal),
	}
	worldT2 := Triangle{
		p0:     TransformPoint(q2, tr2, t2.p0),
		p1:     TransformPoint(q2, tr2, t2.p1),
		p2:     TransformPoint(q2, tr2, t2.p2),
		normal: transformDirection(q2, t2.normal),
	}
	collides, _ := worldT1.collidesWithTriangle(&worldT2, collisionBufferMM)
	return collides
}

// collidesWithGeometryBVH uses BVH to accelerate mesh vs single geometry collision.
//
// On a fresh collision the colliding triangle is stored as a witness keyed by
// the other geometry's label. The matching load happens earlier in
// Mesh.CollidesWith so we can skip the box-vertex sweep on cached hits.
func (m *Mesh) collidesWithGeometryBVH(other Geometry, collisionBufferMM float64) (bool, float64, error) {
	bvh := m.ensureBVH()
	if bvh == nil {
		return false, 0, errors.New("cannot check collision on mesh with no triangles")
	}

	otherMin, otherMax := computeGeometryAABB(other)
	collides, dist, witness, err := bvhCollidesWithGeometryTracked(bvh, m.pose, other, otherMin, otherMax, collisionBufferMM)
	if err != nil {
		return false, 0, err
	}
	if collides && witness != nil && m.state != nil {
		if otherLabel := other.Label(); otherLabel != "" {
			m.state.geomWitness.Store(otherLabel, witness)
		}
	}
	return collides, dist, nil
}

// witnessTriCollidesWith re-checks a cached colliding triangle (from m's BVH)
// against an arbitrary other geometry under fresh poses. Operates on a stack-
// local world-space Triangle so the cache check itself allocates nothing.
func witnessTriCollidesWith(tri *Triangle, meshPose Pose, other Geometry, collisionBufferMM float64) bool {
	q := meshPose.Orientation().Quaternion()
	trans := meshPose.Point()
	worldT := Triangle{
		p0:     TransformPoint(q, trans, tri.p0),
		p1:     TransformPoint(q, trans, tri.p1),
		p2:     TransformPoint(q, trans, tri.p2),
		normal: transformDirection(q, tri.normal),
	}
	collides, _, err := worldT.CollidesWith(other, collisionBufferMM)
	return err == nil && collides
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

// ResetCache clears the mesh's lazily-built collision state: the BVH, the
// deduplicated vertex list, and the witness / negative caches living on the
// shared *meshState. Intended for callers (e.g. the motion planner) that want
// to release the memory held by these caches once a planning session is done.
//
// Because *meshState is shared across Transform-derived copies, clearing it
// here is visible to all clones of this logical mesh. The per-instance bvh
// and uniqueVerts on Transform-derived clones are not reset by this call;
// resetting the original Mesh registered with the frame system is what frees
// the BVH tree memory.
//
// Safe to call concurrently with collision checks — the cache is purely an
// optimization, so racing readers may observe a torn view but cannot return a
// wrong answer (they fall back to the BVH walk).
func (m *Mesh) ResetCache() {
	m.bvh = nil
	m.bvhOnce = sync.Once{}
	m.uniqueVerts = nil
	m.uniqueVertsOnce = sync.Once{}
	if m.state != nil {
		m.state.witnesses = sync.Map{}
		m.state.geomWitness = sync.Map{}
		m.state.negCache = sync.Map{}
	}
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
	m.rawBytesOnce = sync.Once{}
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
