package segmentation

import (
	"context"

	"github.com/pkg/errors"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/vision"
)

// ObjectSegmentation is a struct to store the full point cloud as well as a point cloud array of the objects in the scene.
type ObjectSegmentation struct {
	FullCloud pc.PointCloud
	*Segments
}

// Objects returns the slice of Objects found by object segmentation.
func (objseg *ObjectSegmentation) Objects() []*vision.Object {
	return objseg.Segments.Objects
}

// NewObjectSegmentation removes the planes (if any) and returns a segmentation of the objects in a point cloud.
func NewObjectSegmentation(ctx context.Context, cloud pc.PointCloud, cfg *vision.Parameters3D) (*ObjectSegmentation, error) {
	if cfg == nil {
		return nil, errors.New("config for object segmentation cannot be nil")
	}
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
	segments, err := segmentPointCloudObjects(objCloud, cfg.ClusteringRadiusMm, cfg.MinPtsInSegment)
	if err != nil {
		return nil, err
	}
	objects, err := NewSegmentsFromSlice(segments)
	if err != nil {
		return nil, err
	}
	return &ObjectSegmentation{cloud, objects}, nil
}

// SegmentPointCloudObjects uses radius based nearest neighbors to segment the images, and then prunes away
// segments that do not pass a certain threshold of points.
func segmentPointCloudObjects(cloud pc.PointCloud, radius float64, nMin int) ([]pc.PointCloud, error) {
	segments, err := radiusBasedNearestNeighbors(cloud, radius)
	if err != nil {
		return nil, err
	}
	segments = pc.PrunePointClouds(segments, nMin)
	return segments, nil
}

// RadiusBasedNearestNeighbors partitions the pointcloud, grouping points within a given radius of each other.
// Described in the paper "A Clustering Method for Efficient Segmentation of 3D Laser Data" by Klasing et al. 2008.
func radiusBasedNearestNeighbors(cloud pc.PointCloud, radius float64) ([]pc.PointCloud, error) {
	kdt, ok := cloud.(*pc.KDTree)
	if !ok {
		kdt = pc.NewKDTree(cloud)
	}
	var err error
	clusters := NewSegments()
	c := 0
	kdt.Iterate(func(pt pc.Point) bool {
		v := pt.Position()
		// skip if point already is assigned cluster
		if _, ok := clusters.Indices[v]; ok {
			return true
		}
		// if not assigned, see if any of its neighbors are assigned a cluster
		nn := kdt.RadiusNearestNeighbors(pt, radius, false)
		for _, neighbor := range nn {
			nv := neighbor.Position()
			ptIndex, ptOk := clusters.Indices[v]
			neighborIndex, neighborOk := clusters.Indices[nv]
			switch {
			case ptOk && neighborOk:
				if ptIndex != neighborIndex {
					err = clusters.MergeClusters(ptIndex, neighborIndex)
				}
			case !ptOk && neighborOk:
				err = clusters.AssignCluster(pt, neighborIndex)
			case ptOk && !neighborOk:
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
			for _, neighbor := range nn {
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
