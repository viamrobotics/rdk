package spatialmath


//~ import (
//~ "github.com/golang/geo/r3"
//~ )

// This file incorporates work covered by the Brax project -- https://github.com/google/brax/blob/main/LICENSE.
// Copyright 2021 The Brax Authors, which is licensed under the Apache License Version 2.0 (the “License”).
// You may obtain a copy of the license at http://www.apache.org/licenses/LICENSE-2.0.

// mesh is a collision geometry that represents a set of triangles that represent a mesh.
type Mesh struct {
	pose      Pose
	triangles []*Triangle
}

func NewMesh(pose Pose, triangles []*Triangle) *Mesh {
	return &Mesh{
		pose:      pose,
		triangles: triangles,
	}
}

func (m *Mesh) Pose() Pose {
	return m.pose
}

func (m *Mesh) Triangles() []*Triangle {
	return m.triangles
}

func (m *Mesh) Transform(pose Pose) *Mesh {
	// Triangle points are in frame of mesh, like the corners of a box, so no need to transform them
	return &Mesh{
		pose:      Compose(pose, m.pose),
		triangles: m.triangles,
	}
}
