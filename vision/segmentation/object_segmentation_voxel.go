package segmentation

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/vision"
)

// NewObjectSegmentationFromVoxelGrid removes the planes (if any) and returns a segmentation of the objects in a point cloud.
func NewObjectSegmentationFromVoxelGrid(
	ctx context.Context,
	vg *pc.VoxelGrid,
	objConfig *vision.Parameters3D,
	planeConfig VoxelGridPlaneConfig,
) (*ObjectSegmentation, error) {
	if objConfig == nil {
		return nil, errors.New("config for object segmentation cannot be nil")
	}
	ps := NewVoxelGridPlaneSegmentation(vg, planeConfig)
	planes, nonPlane, err := ps.FindPlanes(ctx)
	if err != nil {
		return nil, err
	}
	// if there is a found plane in the scene, take the biggest plane, and only save the non-plane points above it
	if len(planes) > 0 {
		nonPlane, _, err = SplitPointCloudByPlane(nonPlane, planes[0])
		if err != nil {
			return nil, err
		}
	}

	objVoxGrid := pc.NewVoxelGridFromPointCloud(nonPlane, vg.VoxelSize(), vg.Lambda())
	objects, err := voxelBasedNearestNeighbors(objVoxGrid, objConfig.ClusteringRadiusMm)
	if err != nil {
		return nil, err
	}
	objects = pc.PrunePointClouds(objects, objConfig.MinPtsInSegment)
	segments := NewSegmentsFromSlice(objects)

	return &ObjectSegmentation{nonPlane, segments}, nil
}

func voxelBasedNearestNeighbors(vg *pc.VoxelGrid, radius float64) ([]pc.PointCloud, error) {
	var err error
	vSize := vg.VoxelSize()
	clusters := NewSegments()
	c := 0
	for coord, vox := range vg.Voxels {
		v := pc.Vec3{float64(coord.I), float64(coord.J), float64(coord.K)}
		// skip if point already is assigned cluster
		if _, ok := clusters.Indices[v]; ok {
			continue
		}
		// if not assigned, see if any of its neighbors are assigned a cluster
		n := int((radius - 0.5*vSize) / vSize)
		if n < 1 {
			return nil, fmt.Errorf("cannot use radius %v to cluster voxels of size %v", radius, vSize)
		}
		nn := findVoxelNeighborsInRadius(vg, coord, uint(n))
		for nv, neighborVox := range nn {
			ptIndex, ptOk := clusters.Indices[v]
			neighborIndex, neighborOk := clusters.Indices[nv]
			switch {
			case ptOk && neighborOk:
				if ptIndex != neighborIndex {
					err = clusters.MergeClusters(ptIndex, neighborIndex)
					if err != nil {
						return nil, err
					}
				}
			case !ptOk && neighborOk:
				clusters.Indices[v] = neighborIndex // label the voxel coordinate
				for _, p := range vox.Points {
					err = clusters.AssignCluster(p, neighborIndex) // label all points in the voxel
					if err != nil {
						return nil, err
					}
				}
			case ptOk && !neighborOk:
				clusters.Indices[nv] = ptIndex
				for _, p := range neighborVox.Points {
					err = clusters.AssignCluster(p, ptIndex)
					if err != nil {
						return nil, err
					}
				}
			}
		}
		// if none of the neighbors were assigned a cluster, create a new cluster and assign all neighbors to it
		if _, ok := clusters.Indices[v]; !ok {
			clusters.Indices[v] = c
			for _, p := range vox.Points {
				err = clusters.AssignCluster(p, c)
				if err != nil {
					return nil, err
				}
			}
			for nv, neighborVox := range nn {
				clusters.Indices[nv] = c
				for _, p := range neighborVox.Points {
					err = clusters.AssignCluster(p, c)
					if err != nil {
						return nil, err
					}
				}
			}
			c++
		}
	}
	return clusters.PointClouds(), nil
}

func findVoxelNeighborsInRadius(vg *pc.VoxelGrid, coord pc.VoxelCoords, n uint) map[pc.Vec3]*pc.Voxel {
	neighbors := make(map[pc.Vec3]*pc.Voxel)
	adjCoords := vg.GetNNearestVoxels(vg.Voxels[coord], n)
	for _, c := range adjCoords {
		loc := pc.Vec3{float64(c.I), float64(c.J), float64(c.K)}
		neighbors[loc] = vg.Voxels[c]
	}
	return neighbors
}
