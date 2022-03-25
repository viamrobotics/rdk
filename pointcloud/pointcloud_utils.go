package pointcloud

import (
	"image/color"
	"math"

	"github.com/golang/geo/r3"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/pkg/errors"
	"gonum.org/v1/gonum/stat"

	"go.viam.com/rdk/spatialmath"
)

// MergePointClouds takes a slice of points clouds and adds all their points to one point cloud.
func MergePointClouds(clouds []PointCloud) (PointCloud, error) {
	if len(clouds) == 0 {
		return nil, errors.New("no point clouds to merge")
	}
	if len(clouds) == 1 {
		return clouds[0], nil
	}
	merged := New()
	var err error
	for _, c := range clouds {
		c.Iterate(func(pt Point) bool {
			err = merged.Set(pt)
			return err == nil
		})
		if err != nil {
			return nil, err
		}
	}
	return merged, nil
}

// MergePointCloudsWithColor creates a union of point clouds from the slice of point clouds, giving
// each element of the slice a unique color.
func MergePointCloudsWithColor(clusters []PointCloud) (PointCloud, error) {
	var err error
	palette := colorful.FastWarmPalette(len(clusters))
	colorSegmentation := New()
	for i, cluster := range clusters {
		col := color.NRGBAModel.Convert(palette[i])
		cluster.Iterate(func(pt Point) bool {
			v := pt.Position()
			colorPoint := NewColoredPoint(v.X, v.Y, v.Z, col.(color.NRGBA))
			err = colorSegmentation.Set(colorPoint)
			return err == nil
		})
		if err != nil {
			return nil, err
		}
	}
	return colorSegmentation, nil
}

// BoundingBoxFromPointCloud returns a Geometry object that encompasses all the points in the given point cloud.
func BoundingBoxFromPointCloud(cloud PointCloud) (spatialmath.Geometry, error) {
	if cloud.Size() == 0 {
		return nil, nil
	}

	// calculate extents of point cloud
	dims := r3.Vector{math.Abs(cloud.MaxX() - cloud.MinX()), math.Abs(cloud.MaxY() - cloud.MinY()), math.Abs(cloud.MaxZ() - cloud.MinZ())}

	// calculate the spatial average center of a given point cloud
	x, y, z := 0.0, 0.0, 0.0
	n := float64(cloud.Size())
	cloud.Iterate(func(pt Point) bool {
		v := pt.Position()
		x += v.X
		y += v.Y
		z += v.Z
		return true
	})
	mean := r3.Vector{x / n, y / n, z / n}

	// calculate the dimensions of the bounding box formed by finding the dimensions of each axes' extrema
	return spatialmath.NewBox(spatialmath.NewPoseFromPoint(mean), dims)
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
			kd = NewKDTree(pc)
		}
		// get the statistical information
		avgDistances := make([]float64, 0, kd.Size())
		points := make([]Point, 0, kd.Size())
		kd.Iterate(func(pt Point) bool {
			neighbors := kd.KNearestNeighbors(pt, meanK, false)
			sumDist := 0.0
			for _, p := range neighbors {
				sumDist += pt.Distance(p)
			}
			avgDistances = append(avgDistances, sumDist/float64(len(neighbors)))
			points = append(points, pt)
			return true
		})
		mean, stddev := stat.MeanStdDev(avgDistances, nil)
		threshold := mean + stdDevThresh*stddev
		// filter using the statistical information
		filteredCloud := New()
		for i := 0; i < kd.Size(); i++ {
			if avgDistances[i] < threshold {
				err := filteredCloud.Set(points[i])
				if err != nil {
					return nil, err
				}
			}
		}
		return filteredCloud, nil
	}
	return filterFunc, nil
}
