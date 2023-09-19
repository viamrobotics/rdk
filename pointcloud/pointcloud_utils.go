package pointcloud

import (
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"gonum.org/v1/gonum/stat"

	"go.viam.com/rdk/spatialmath"
)

// BoundingBoxFromPointCloud returns a Geometry object that encompasses all the points in the given point cloud.
func BoundingBoxFromPointCloud(cloud PointCloud) (spatialmath.Geometry, error) {
	return BoundingBoxFromPointCloudWithLabel(cloud, "")
}

// BoundingBoxFromPointCloudWithLabel returns a Geometry object that encompasses all the points in the given point cloud.
func BoundingBoxFromPointCloudWithLabel(cloud PointCloud, label string) (spatialmath.Geometry, error) {
	if cloud.Size() == 0 {
		return nil, nil
	}

	// calculate extents of point cloud
	meta := cloud.MetaData()
	dims := r3.Vector{math.Abs(meta.MaxX - meta.MinX), math.Abs(meta.MaxY - meta.MinY), math.Abs(meta.MaxZ - meta.MinZ)}

	// calculate the spatial average center of a given point cloud
	n := float64(cloud.Size())
	mean := r3.Vector{meta.TotalX() / n, meta.TotalY() / n, meta.TotalZ() / n}

	// calculate the dimensions of the bounding box formed by finding the dimensions of each axes' extrema
	return spatialmath.NewBox(spatialmath.NewPoseFromPoint(mean), dims, label)
}

// PrunePointClouds removes point clouds from a slice if the point cloud has less than nMin points.
func PrunePointClouds(clouds []PointCloud, nMin int) []PointCloud {
	pruned := make([]PointCloud, 0, len(clouds))
	for _, cloud := range clouds {
		if cloud.Size() >= nMin {
			pruned = append(pruned, cloud)
		}
	}
	return pruned
}

// StatisticalOutlierFilter implements the function from PCL to remove noisy points from a point cloud.
// https://pcl.readthedocs.io/projects/tutorials/en/latest/statistical_outlier.html
// This returns a function that can be used to filter on point clouds.
// NOTE(bh): Returns a new point cloud, but could be modified to filter and change the original point cloud.
func StatisticalOutlierFilter(meanK int, stdDevThresh float64) (func(PointCloud) (PointCloud, error), error) {
	if meanK <= 0 {
		return nil, errors.Errorf("argument meanK must be a positive int, got %d", meanK)
	}
	if stdDevThresh <= 0.0 {
		return nil, errors.Errorf("argument stdDevThresh must be a positive float, got %.2f", stdDevThresh)
	}
	filterFunc := func(pc PointCloud) (PointCloud, error) {
		// create data type that can do nearest neighbors
		kd, ok := pc.(*KDTree)
		if !ok {
			kd = ToKDTree(pc)
		}
		// get the statistical information
		avgDistances := make([]float64, 0, kd.Size())
		points := make([]PointAndData, 0, kd.Size())
		kd.Iterate(0, 0, func(v r3.Vector, d Data) bool {
			neighbors := kd.KNearestNeighbors(v, meanK, false)
			sumDist := 0.0
			for _, p := range neighbors {
				sumDist += v.Distance(p.P)
			}
			avgDistances = append(avgDistances, sumDist/float64(len(neighbors)))
			points = append(points, PointAndData{v, d})
			return true
		})

		mean, stddev := stat.MeanStdDev(avgDistances, nil)
		threshold := mean + stdDevThresh*stddev
		// filter using the statistical information
		filteredCloud := New()
		for i := 0; i < len(avgDistances); i++ {
			if avgDistances[i] < threshold {
				err := filteredCloud.Set(points[i].P, points[i].D)
				if err != nil {
					return nil, err
				}
			}
		}
		return filteredCloud, nil
	}
	return filterFunc, nil
}
