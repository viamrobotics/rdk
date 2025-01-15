package spatialmath

// This file incorporates work covered by the Brax project -- https://github.com/google/brax/blob/main/LICENSE.
// Copyright 2021 The Brax Authors, which is licensed under the Apache License Version 2.0 (the “License”).
// You may obtain a copy of the license at http://www.apache.org/licenses/LICENSE-2.0.

// Mesh is a set of triangles at some pose. Triangle points are in the frame of the mesh.
type Mesh struct {
	pose      Pose
	triangles []*Triangle
}

// NewMesh creates a mesh from the given triangles and pose.
func NewMesh(pose Pose, triangles []*Triangle) *Mesh {
	return &Mesh{
		pose:      pose,
		triangles: triangles,
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
func (m *Mesh) Transform(pose Pose) *Mesh {
	// Triangle points are in frame of mesh, like the corners of a box, so no need to transform them
	return &Mesh{
		pose:      Compose(pose, m.pose),
		triangles: m.triangles,
	}
}
