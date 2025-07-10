package pointcloud

import (
	"github.com/golang/geo/r3"
)

// MakeTestPointCloud creates a test point cloud with 3 points.
func MakeTestPointCloud(label string) *BasicOctree {
	pc := NewBasicPointCloud(3)
	err := pc.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewBasicData())
	if err != nil {
		return nil
	}
	err = pc.Set(r3.Vector{X: 1, Y: 0, Z: 0}, NewBasicData())
	if err != nil {
		return nil
	}
	err = pc.Set(r3.Vector{X: 0, Y: 1, Z: 0}, NewBasicData())
	if err != nil {
		return nil
	}

	octree, err := ToBasicOctree(pc, 50)
	if err != nil {
		return nil
	}

	octree.SetLabel(label)
	return octree
}
