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
// A Mesh created this way should not be attempted to be converted to protobuf
// as there are not conversion functions to support it currently.
func NewMesh(pose Pose, triangles []*Triangle, label string) *Mesh {
	// TODO(RSDK-10314): Fix proto for meshes created from triangles.
	return &Mesh{
		pose:      pose,
		triangles: triangles,
		label:     label,
	}
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
	triangles := []*Triangle{}
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

func newMeshFromProto(pose Pose, m *commonpb.Mesh, label string) (*Mesh, error) {
	switch m.ContentType {
	case string(plyType):
		return newMeshFromBytes(pose, m.Mesh, label)
	default:
		return nil, fmt.Errorf("unsupported Mesh type: %s", m.ContentType)
	}
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
		return m.collidesWithSphere(other.position, 0, collisionBufferMM), nil
	case *sphere:
		return m.collidesWithSphere(other.pose.Point(), other.radius, collisionBufferMM), nil
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
		return m.distanceFromSphere(other.position, 0), nil
	case *sphere:
		return m.distanceFromSphere(other.pose.Point(), other.radius), nil
	case *Mesh:
		return m.distanceFromMesh(other), nil
	default:
		return math.Inf(-1), newCollisionTypeUnsupportedError(m, g)
	}
}

// Returns true if any triangle vertex of the mesh intersects the box.
func (m *Mesh) boxIntersectsVertex(b *box) bool {
	for _, p := range m.ToPoints(1) {
		if pointVsBoxCollision(p, b, defaultCollisionBufferMM) {
			return true
		}
	}
	return false
}

func (m *Mesh) distanceFromSphere(pt r3.Vector, radius float64) float64 {
	minDist := math.Inf(1)

	for _, tri := range m.triangles {
		worldTri := NewTriangle(
			Compose(m.pose, NewPoseFromPoint(tri.p0)).Point(),
			Compose(m.pose, NewPoseFromPoint(tri.p1)).Point(),
			Compose(m.pose, NewPoseFromPoint(tri.p2)).Point(),
		)
		closestPt := ClosestPointTrianglePoint(worldTri, pt)
		dist := closestPt.Sub(pt).Norm() - radius
		if dist < minDist {
			minDist = dist
		}
	}
	return minDist
}

func (m *Mesh) collidesWithSphere(pt r3.Vector, radius, buffer float64) bool {
	// Transform all triangles to world space once
	for _, tri := range m.triangles {
		worldTri := NewTriangle(
			Compose(m.pose, NewPoseFromPoint(tri.p0)).Point(),
			Compose(m.pose, NewPoseFromPoint(tri.p1)).Point(),
			Compose(m.pose, NewPoseFromPoint(tri.p2)).Point(),
		)
		closestPt := ClosestPointTrianglePoint(worldTri, pt)
		if closestPt.Sub(pt).Norm() <= radius+buffer {
			return true
		}
	}
	return false
}

// collidesWithMesh checks if this mesh collides with another mesh
// TODO: This function is *begging* for GPU acceleration.
func (m *Mesh) collidesWithMesh(other *Mesh, collisionBufferMM float64) bool {
	// Transform all triangles to world space once
	worldTris1 := make([]*Triangle, len(m.triangles))
	for i, tri := range m.triangles {
		worldTris1[i] = NewTriangle(
			Compose(m.pose, NewPoseFromPoint(tri.p0)).Point(),
			Compose(m.pose, NewPoseFromPoint(tri.p1)).Point(),
			Compose(m.pose, NewPoseFromPoint(tri.p2)).Point(),
		)
	}

	worldTris2 := make([]*Triangle, len(other.triangles))
	for i, tri := range other.triangles {
		worldTris2[i] = NewTriangle(
			Compose(other.pose, NewPoseFromPoint(tri.p0)).Point(),
			Compose(other.pose, NewPoseFromPoint(tri.p1)).Point(),
			Compose(other.pose, NewPoseFromPoint(tri.p2)).Point(),
		)
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
				bestSegPt, bestTriPt := closestPointsSegmentTriangle(start, end, worldTri2)
				if bestSegPt.Sub(bestTriPt).Norm() <= collisionBufferMM {
					return true
				}
			}

			// Check segments from tri2 against tri1
			for i := 0; i < 3; i++ {
				start := p2[i]
				end := p2[(i+1)%3]
				bestSegPt, bestTriPt := closestPointsSegmentTriangle(start, end, worldTri1)
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
	// Transform all triangles to world space once
	worldTris1 := make([]*Triangle, len(m.triangles))
	for i, tri := range m.triangles {
		worldTris1[i] = NewTriangle(
			Compose(m.pose, NewPoseFromPoint(tri.p0)).Point(),
			Compose(m.pose, NewPoseFromPoint(tri.p1)).Point(),
			Compose(m.pose, NewPoseFromPoint(tri.p2)).Point(),
		)
	}

	worldTris2 := make([]*Triangle, len(other.triangles))
	for i, tri := range other.triangles {
		worldTris2[i] = NewTriangle(
			Compose(other.pose, NewPoseFromPoint(tri.p0)).Point(),
			Compose(other.pose, NewPoseFromPoint(tri.p1)).Point(),
			Compose(other.pose, NewPoseFromPoint(tri.p2)).Point(),
		)
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
				bestSegPt, bestTriPt := closestPointsSegmentTriangle(start, end, worldTri2)
				dist := bestSegPt.Sub(bestTriPt).Norm()
				if dist < minDist {
					minDist = dist
				}
			}

			// Check segments from tri2 against tri1
			for i := 0; i < 3; i++ {
				start := p2[i]
				end := p2[(i+1)%3]
				bestSegPt, bestTriPt := closestPointsSegmentTriangle(start, end, worldTri1)
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
func (m *Mesh) ToPoints(density float64) []r3.Vector {
	// Use map to deduplicate vertices
	pointMap := make(map[string]r3.Vector)

	// Add all triangle vertices, formatting as a string for map deduplication
	for _, tri := range m.triangles {
		for _, pt := range tri.Points() {
			// Transform point to world space
			worldPt := Compose(m.pose, NewPoseFromPoint(pt)).Point()
			key := fmt.Sprintf("%.10f,%.10f,%.10f", worldPt.X, worldPt.Y, worldPt.Z)
			pointMap[key] = worldPt
		}
	}

	// Convert map back to slice
	points := make([]r3.Vector, 0, len(pointMap))
	for _, pt := range pointMap {
		points = append(points, pt)
	}
	return points
}

// MarshalJSON implements the json.Marshaler interface.
func (m *Mesh) MarshalJSON() ([]byte, error) {
	return nil, errors.New("MarshalJSON not yet implemented for Mesh")
}
