package segmentation

import (
	"image/color"

	pc "go.viam.com/core/pointcloud"
	"go.viam.com/core/utils"

	"github.com/golang/geo/r3"
	"github.com/lucasb-eyer/go-colorful"
)

// ColorPointCloudSegments creates a union of point clouds from the slice of point clouds, giving
// each element of the slice a unique color.
func ColorPointCloudSegments(clusters []pc.PointCloud) (pc.PointCloud, error) {
	var err error
	palette := colorful.FastWarmPalette(len(clusters))
	colorSegmentation := pc.New()
	for i, cluster := range clusters {
		col := color.NRGBAModel.Convert(palette[i])
		cluster.Iterate(func(pt pc.Point) bool {
			v := pt.Position()
			colorPoint := pc.NewColoredPoint(v.X, v.Y, v.Z, col.(color.NRGBA))
			err = colorSegmentation.Set(colorPoint)
			return err == nil
		})
		if err != nil {
			return nil, err
		}
	}
	return colorSegmentation, nil
}

// GetMeanCenterOfPointCloud returns the spatial average center of a given point cloud.
func GetMeanCenterOfPointCloud(cloud pc.PointCloud) r3.Vector {
	x, y, z := 0.0, 0.0, 0.0
	n := float64(cloud.Size())
	cloud.Iterate(func(pt pc.Point) bool {
		v := pt.Position()
		x += v.X
		y += v.Y
		z += v.Z
		return true
	})
	return r3.Vector{x / n, y / n, z / n}
}

// SegmentPointCloudObjects uses radius based nearest neighbors to segment the images, and then prunes away
// segments that do not pass a certain threshold of points
func SegmentPointCloudObjects(cloud pc.PointCloud, radius float64, nMin int) ([]pc.PointCloud, error) {
	segments, err := RadiusBasedNearestNeighbors(cloud, radius)
	if err != nil {
		return nil, err
	}
	segments = pruneClusters(segments, nMin)
	return segments, nil
}

// RadiusBasedNearestNeighbors partitions the pointcloud, grouping points within a given radius of each other.
// Described in the paper "A Clustering Method for Efficient Segmentation of 3D Laser Data" by Klasing et al. 2008
func RadiusBasedNearestNeighbors(cloud pc.PointCloud, radius float64) ([]pc.PointCloud, error) {
	var err error
	clusterAssigned := make(map[pc.Vec3]int)
	clusters := make([]pc.PointCloud, 0)
	c := 0
	cloud.Iterate(func(pt pc.Point) bool {
		v := pt.Position()
		// skip if point already is assigned cluster
		if _, ok := clusterAssigned[v]; ok {
			return true
		}
		// if not assigned, see if any of its neighbors are assigned a cluster
		nn := findNeighborsInRadius(cloud, pt, radius)
		for neighbor := range nn {
			nv := neighbor.Position()
			ptIndex, ptOk := clusterAssigned[v]
			neighborIndex, neighborOk := clusterAssigned[nv]
			if ptOk && neighborOk {
				if ptIndex != neighborIndex {
					clusters, err = mergeClusters(ptIndex, neighborIndex, clusters, clusterAssigned)
				}
			} else if !ptOk && neighborOk {
				clusters, err = assignCluster(pt, neighborIndex, clusters)
				clusterAssigned[v] = neighborIndex
			} else if ptOk && !neighborOk {
				clusters, err = assignCluster(neighbor, ptIndex, clusters)
				clusterAssigned[nv] = ptIndex
			}
			if err != nil {
				return false
			}
		}
		// if none of the neighbors were assigned a cluster, create a new cluster and assign all neighbors to it
		if _, ok := clusterAssigned[v]; !ok {
			clusterAssigned[v] = c
			clusters, err = assignCluster(pt, c, clusters)
			if err != nil {
				return false
			}
			for neighbor := range nn {
				clusterAssigned[neighbor.Position()] = c
				clusters, err = assignCluster(neighbor, c, clusters)
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
	return clusters, nil
}

// this is pretty inefficient since it has to loop through all the points in the pointcloud
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

func assignCluster(point pc.Point, index int, clusters []pc.PointCloud) ([]pc.PointCloud, error) {
	for index >= len(clusters) {
		clusters = append(clusters, pc.New())
	}
	err := clusters[index].Set(point)
	if err != nil {
		return nil, err
	}
	return clusters, nil
}

func mergeClusters(from, to int, clusters []pc.PointCloud, clusterMap map[pc.Vec3]int) ([]pc.PointCloud, error) {
	var err error
	index := utils.MaxInt(from, to)
	for index >= len(clusters) {
		clusters = append(clusters, pc.New())
	}
	clusters[from].Iterate(func(pt pc.Point) bool {
		v := pt.Position()
		clusterMap[v] = to
		err = clusters[to].Set(pt)
		clusters[from].Unset(v.X, v.Y, v.Z)
		return err == nil
	})
	return clusters, err
}

func pruneClusters(clusters []pc.PointCloud, nMin int) []pc.PointCloud {
	pruned := make([]pc.PointCloud, 0, len(clusters))
	for _, cluster := range clusters {
		if cluster.Size() >= nMin {
			pruned = append(pruned, cluster)
		}
	}
	return pruned
}
