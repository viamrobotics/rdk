package pointcloud

import (
	"math"
	"sync"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"gonum.org/v1/gonum/stat"

	"go.viam.com/rdk/spatialmath"
)

// var iterateMutex sync.Mutex

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
	x, y, z := 0.0, 0.0, 0.0
	n := float64(cloud.Size())
	cloud.Iterate(0, 0, func(v r3.Vector, d Data) bool {
		x += v.X
		y += v.Y
		z += v.Z
		return true
	})
	mean := r3.Vector{x / n, y / n, z / n}

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
		pointAsMatrix, ok := kd.points.(*matrixStorage)
		if ok {
			c1 := make(chan float64, kd.Size())
			c2 := make(chan PointAndData, kd.Size())
			var newWG sync.WaitGroup
			// fmt.Println("iterating concurrently")
			newWG.Add(1)
			go func() {
				defer newWG.Done()
				distDone := false
				pointDone := false
				for {
					select {
					case newDist, isDone := <-c1:
						if isDone {
							// fmt.Println("dist done")
							distDone = isDone
							continue
						}
						avgDistances = append(avgDistances, newDist)
					case newPoint, isDone := <-c2:
						if isDone {
							// fmt.Println("point done")
							pointDone = isDone
							continue
						}
						points = append(points, newPoint)
					default:
						if distDone && pointDone {
							return
						}
						// case isDone := <-c3:
						// 	if isDone {
						// 		fmt.Println("done")
						// 		return
						// 	}
					}
				}
			}()
			pointAsMatrix.IterateConcurrently(4, func(v r3.Vector, d Data) bool {
				neighbors := kd.KNearestNeighbors(v, meanK, false)
				sumDist := 0.0
				for _, p := range neighbors {
					sumDist += v.Distance(p.P)
				}
				// iterateMutex.Lock()
				c1 <- (sumDist / float64(len(neighbors)))
				c2 <- PointAndData{v, d}
				// fmt.Println(len(c2))
				// avgDistances = append(avgDistances, sumDist/float64(len(neighbors)))
				// points = append(points, PointAndData{v, d})
				// iterateMutex.Unlock()
				return true
			},
			)
			close(c1)
			close(c2)
			newWG.Wait()
			c1Len := len(c1)
			c2Len := len(c2)
			// fmt.Println("kd sis:", kd.Size(), "avgdists:", len(c1), "points", len(c2))
			if float64(kd.Size())*0.97 > float64(c1Len) || float64(kd.Size())*0.97 > float64(c2Len) {
				panic("too many points lost in concurrency issues")
			}
			for i := range c1 {
				avgDistances = append(avgDistances, i)
			}
			// for i := range c2 {
			// 	points = append(points, i)
			// }

			// fmt.Println("kd size:", kd.Size(), "avgdists:", len(avgDistances), "points", len(points))
			// fmt.Println("avg distances", avgDistances)

			mean, stddev := stat.MeanStdDev(avgDistances, nil)
			threshold := mean + stdDevThresh*stddev
			filteredCloud := New()
			i := 0
			for curPoint := range c2 {
				if avgDistances[i] < threshold {
					err := filteredCloud.Set(curPoint.P, curPoint.D)
					if err != nil {
						return nil, err
					}
				}
				i++
				if i >= c1Len {
					break
				}
			}
			return filteredCloud, nil
		} else {
			// fmt.Println("iterating singularly")
			kd.points.Iterate(0, 0, func(v r3.Vector, d Data) bool {
				neighbors := kd.KNearestNeighbors(v, meanK, false)
				sumDist := 0.0
				for _, p := range neighbors {
					sumDist += v.Distance(p.P)
				}
				avgDistances = append(avgDistances, sumDist/float64(len(neighbors)))
				points = append(points, PointAndData{v, d})
				return true
			})
			// fmt.Println("avg distances", avgDistances)

			mean, stddev := stat.MeanStdDev(avgDistances, nil)
			threshold := mean + stdDevThresh*stddev
			// fmt.Println(mean, stddev, threshold)
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
	}
	return filterFunc, nil
}
