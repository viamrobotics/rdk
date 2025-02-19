package spatialmath

import (
	"errors"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
)

// This file incorporates work covered by the Brax project -- https://github.com/google/brax/blob/main/LICENSE.
// Copyright 2021 The Brax Authors, which is licensed under the Apache License Version 2.0 (the "License").
// You may obtain a copy of the license at http://www.apache.org/licenses/LICENSE-2.0.

// Mesh is a set of triangles at some pose. Triangle points are in the frame of the mesh.
type Mesh struct {
	pose      Pose
	triangles []*Triangle
	label     string
}

// NewMesh creates a mesh from the given triangles and pose.
func NewMesh(pose Pose, triangles []*Triangle, label string) *Mesh {
	return &Mesh{
		pose:      pose,
		triangles: triangles,
		label:     label,
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
	}
}

// CollidesWith checks if the given mesh collides with the given geometry and returns true if it does.
func (m *Mesh) CollidesWith(g Geometry, collisionBufferMM float64) (bool, error) {
	switch other := g.(type) {
	case *box:
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
	var points []r3.Vector
	for _, tri := range m.triangles {
		points = append(points, tri.Points()...)
	}

	for _, pt := range points {
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

func (m *Mesh) distanceFromSphere(pt r3.Vector, radius float64) float64 {
	minDist := math.Inf(1)
	for _, tri := range m.triangles {
		closestPt := ClosestPointTrianglePoint(tri, pt)
		dist := closestPt.Sub(pt).Norm() - radius
		if dist < minDist {
			minDist = dist
		}
	}
	return minDist
}

func (m *Mesh) collidesWithSphere(pt r3.Vector, radius, buffer float64) bool {
	for _, tri := range m.triangles {
		closestPt := ClosestPointTrianglePoint(tri, pt)
		if closestPt.Sub(pt).Norm() <= radius+buffer {
			return true
		}
	}
	return false
}

// collidesWithMesh checks if this mesh collides with another mesh
// TODO: This function is *begging* for GPU acceleration.
func (m *Mesh) collidesWithMesh(other *Mesh, collisionBufferMM float64) bool {
	// Check if any triangles from either mesh collide.
	// If two triangles intersect, then the segment between two vertices of one triangle intersects the other triangle.
	for _, tri1 := range m.triangles {
		for _, tri2 := range other.triangles {
			p1 := tri1.Points()
			for i := 0; i < 3; i++ {
				// This is sufficiently perf sensitive that it is better to avoid an extra function call here
				start := p1[i]
				end := p1[(i+1)%3]
				bestSegPt, bestTriPt := closestPointsSegmentTriangle(start, end, tri1)
				if bestSegPt.Sub(bestTriPt).Norm() <= collisionBufferMM {
					return true
				}
			}
			p2 := tri2.Points()
			for i := 0; i < 3; i++ {
				start := p2[i]
				end := p2[(i+1)%3]
				bestSegPt, bestTriPt := closestPointsSegmentTriangle(start, end, tri1)
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
	minDist := math.Inf(1)
	for _, tri1 := range m.triangles {
		for _, tri2 := range other.triangles {
			p1 := tri1.Points()
			for i := 0; i < 3; i++ {
				// This is sufficiently perf sensitive that it is better to avoid an extra function call here
				start := p1[i]
				end := p1[(i+1)%3]
				bestSegPt, bestTriPt := closestPointsSegmentTriangle(start, end, tri1)
				dist := bestSegPt.Sub(bestTriPt).Norm()
				if dist < minDist {
					minDist = dist
				}
			}
			p2 := tri2.Points()
			for i := 0; i < 3; i++ {
				start := p2[i]
				end := p2[(i+1)%3]
				bestSegPt, bestTriPt := closestPointsSegmentTriangle(start, end, tri1)
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
			key := fmt.Sprintf("%.10f,%.10f,%.10f", pt.X, pt.Y, pt.Z)
			pointMap[key] = pt
		}
	}

	// Convert map back to slice
	points := make([]r3.Vector, 0, len(pointMap))
	for _, pt := range pointMap {
		points = append(points, pt)
	}
	return points
}

// ToProtobuf converts a Mesh to its protobuf representation.
// Note: Since there's no direct mesh representation in the common proto,
// we'll convert it to a collection of triangles as points.
func (m *Mesh) ToProtobuf() *commonpb.Geometry {
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (m *Mesh) MarshalJSON() ([]byte, error) {
	return nil, errors.New("MarshalJSON not yet implemented for Mesh")
}
