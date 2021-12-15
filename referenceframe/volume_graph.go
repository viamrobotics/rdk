package referenceframe

import (
	spatial "go.viam.com/core/spatialmath"
)

type VolumeGraph struct {
	nodes       []volumeNode
	adjacencies []bool
}

type volumeNode struct {
	name   string
	volume spatial.Volume
}

func NewVolumeGraph(nodes []volumeNode) *VolumeGraph {
	vg := &VolumeGraph{}
	vg.nodes = nodes
	vg.adjacencies = make([]bool, len(nodes)*len(nodes))
	return vg
}
