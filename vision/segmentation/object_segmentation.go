package segmentation

import (
	"fmt"

	pc "go.viam.com/core/pointcloud"

	"github.com/golang/geo/r3"
)

// ObjectSegmentation is a struct to store the full point cloud as well as a point cloud array of the objects in the scene.
type ObjectSegmentation struct {
	FullCloud pc.PointCloud
	*Clusters
}

// N gives the number of found segments.
func (objectSeg *ObjectSegmentation) N() int {
	return len(objectSeg.PointClouds)
}

// SelectSegmentFromPoint takes a 3D point as input and outputs the point cloud of the object that the point belongs to.
// returns the full point cloud if the point is not part of any object segment.
func (objectSeg *ObjectSegmentation) SelectSegmentFromPoint(x, y, z float64) (pc.PointCloud, error) {
	v := pc.Vec3{x, y, z}
	if segIndex, ok := objectSeg.Indices[v]; ok {
		return objectSeg.PointClouds[segIndex], nil
	}
	return nil, fmt.Errorf("no segment found at point (%v, %v, %v)", x, y, z)
}

// CreateObjectSegmentation removes the planes (if any) and returns a segmentation of the objects in a point cloud
func CreateObjectSegmentation(cloud pc.PointCloud, minPtsInPlane, minPtsInSegment int, clusteringRadius float64) (*ObjectSegmentation, error) {
	planes, nonPlane, err := FindPlanesInPointCloud(cloud, 10, minPtsInPlane)
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
	segments, err := SegmentPointCloudObjects(objCloud, clusteringRadius, minPtsInSegment)
	if err != nil {
		return nil, err
	}
	clusters := NewClustersFromSlice(segments)
	return &ObjectSegmentation{cloud, clusters}, nil
}

// SegmentPointCloudObjects uses radius based nearest neighbors to segment the images, and then prunes away
// segments that do not pass a certain threshold of points
func SegmentPointCloudObjects(cloud pc.PointCloud, radius float64, nMin int) ([]pc.PointCloud, error) {
	segments, err := RadiusBasedNearestNeighbors(cloud, radius)
	if err != nil {
		return nil, err
	}
	segments = pc.PrunePointClouds(segments, nMin)
	return segments, nil
}

// RadiusBasedNearestNeighbors partitions the pointcloud, grouping points within a given radius of each other.
// Described in the paper "A Clustering Method for Efficient Segmentation of 3D Laser Data" by Klasing et al. 2008
func RadiusBasedNearestNeighbors(cloud pc.PointCloud, radius float64) ([]pc.PointCloud, error) {
	var err error
	clusters := NewClusters()
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
	return clusters.PointClouds, nil
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
