package segmentation

import (
	"context"
	"fmt"

	pc "go.viam.com/core/pointcloud"

	"github.com/golang/geo/r3"
)

// ObjectConfig specifies the necessary parameters for object segmentation
type ObjectConfig struct {
	MinPtsInPlane    int
	MinPtsInSegment  int
	ClusteringRadius float64
}

// ObjectSegmentation is a struct to store the full point cloud as well as a point cloud array of the objects in the scene.
type ObjectSegmentation struct {
	FullCloud pc.PointCloud
	*Segments
}

// NewObjectSegmentation removes the planes (if any) and returns a segmentation of the objects in a point cloud
func NewObjectSegmentation(ctx context.Context, cloud pc.PointCloud, cfg ObjectConfig) (*ObjectSegmentation, error) {
	ps := NewPointCloudPlaneSegmentation(cloud, 10, cfg.MinPtsInPlane)
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
	objCloud, err := pc.NewRoundingPointCloudFromPC(nonPlane)
	if err != nil {
		return nil, err
	}
	segments, err := segmentPointCloudObjects(objCloud, cfg.ClusteringRadius, cfg.MinPtsInSegment)
	if err != nil {
		return nil, err
	}
	objects := NewSegmentsFromSlice(segments)
	return &ObjectSegmentation{cloud, objects}, nil
}

// SegmentPointCloudObjects uses radius based nearest neighbors to segment the images, and then prunes away
// segments that do not pass a certain threshold of points
func segmentPointCloudObjects(cloud pc.PointCloud, radius float64, nMin int) ([]pc.PointCloud, error) {
	segments, err := radiusBasedNearestNeighbors(cloud, radius)
	if err != nil {
		return nil, err
	}
	segments = pc.PrunePointClouds(segments, nMin)
	return segments, nil
}

// RadiusBasedNearestNeighbors partitions the pointcloud, grouping points within a given radius of each other.
// Described in the paper "A Clustering Method for Efficient Segmentation of 3D Laser Data" by Klasing et al. 2008
func radiusBasedNearestNeighbors(cloud pc.PointCloud, radius float64) ([]pc.PointCloud, error) {
	var err error
	clusters := NewSegments()
	c := 0
	cloud.Iterate(func(pt pc.Point) bool {
		v := pt.Position()
		// skip if point already is assigned cluster
		if _, ok := clusters.Indices[v]; ok {
			return true
		}
		// if not assigned, see if any of its neighbors are assigned a cluster
		nn := findNeighborsInRadius(cloud, pt, radius)
		for neighbor := range nn {
			nv := neighbor.Position()
			ptIndex, ptOk := clusters.Indices[v]
			neighborIndex, neighborOk := clusters.Indices[nv]
			if ptOk && neighborOk {
				if ptIndex != neighborIndex {
					err = clusters.MergeClusters(ptIndex, neighborIndex)
				}
			} else if !ptOk && neighborOk {
				err = clusters.AssignCluster(pt, neighborIndex)
			} else if ptOk && !neighborOk {
				err = clusters.AssignCluster(neighbor, ptIndex)
			}
			if err != nil {
				return false
			}
		}
		// if none of the neighbors were assigned a cluster, create a new cluster and assign all neighbors to it
		if _, ok := clusters.Indices[v]; !ok {
			err = clusters.AssignCluster(pt, c)
			if err != nil {
				return false
			}
			for neighbor := range nn {
				err = clusters.AssignCluster(neighbor, c)
				if err != nil {
					return false
				}
			}
			c++
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return clusters.PointClouds(), nil
}

// this is pretty inefficient since it has to loop through all the points in the pointcloud to find only the nearest points.
func findNeighborsInRadius(cloud pc.PointCloud, point pc.Point, radius float64) map[pc.Point]bool {
	neighbors := make(map[pc.Point]bool)
	origin := r3.Vector(point.Position())
	cloud.Iterate(func(pt pc.Point) bool {
		target := r3.Vector(pt.Position())
		d := origin.Distance(target)
		if d == 0.0 {
			return true
		}
		if d <= radius {
			neighbors[pt] = true
		}
		return true
	})
	return neighbors
}

// NewObjectSegmentationFromVoxelGrid removes the planes (if any) and returns a segmentation of the objects in a point cloud
func NewObjectSegmentationFromVoxelGrid(
	ctx context.Context,
	vg *pc.VoxelGrid,
	objConfig ObjectConfig,
	planeConfig VoxelGridPlaneConfig,
) (*ObjectSegmentation, error) {
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
	objects, err := voxelBasedNearestNeighbors(objVoxGrid, objConfig.ClusteringRadius)
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
			if ptOk && neighborOk {
				if ptIndex != neighborIndex {
					err = clusters.MergeClusters(ptIndex, neighborIndex)
					if err != nil {
						return nil, err
					}
				}
			} else if !ptOk && neighborOk {
				clusters.Indices[v] = neighborIndex // label the voxel coordinate
				for _, p := range vox.Points {
					err = clusters.AssignCluster(p, neighborIndex) // label all points in the voxel
					if err != nil {
						return nil, err
					}
				}
			} else if ptOk && !neighborOk {
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
