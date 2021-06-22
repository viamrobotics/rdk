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

// Clusters is a struct for keeping track of the individual segments of a point cloud as they are being built.
// PointClouds is a slice of all the segments, and Indices is a map that assigns each point to the segment index it is a part of.
type Clusters struct {
	PointClouds []pc.PointCloud
	Indices     map[pc.Vec3]int
}

// NewClusters creates an empty new Clusters struct
func NewClusters() *Clusters {
	pointclouds := make([]pc.PointCloud, 0)
	indices := make(map[pc.Vec3]int)
	return &Clusters{pointclouds, indices}
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

// AssignCluster assigns the given point to the cluster with the given index
func (c *Clusters) AssignCluster(point pc.Point, index int) error {
	for index >= len(c.PointClouds) {
		c.PointClouds = append(c.PointClouds, pc.New())
	}
	c.Indices[point.Position()] = index
	err := c.PointClouds[index].Set(point)
	return err
}

// MergeClusters moves all the points in index "from" to the segment at index "to"
func (c *Clusters) MergeClusters(from, to int) error {
	var err error
	index := utils.MaxInt(from, to)
	for index >= len(c.PointClouds) {
		c.PointClouds = append(c.PointClouds, pc.New())
	}
	c.PointClouds[from].Iterate(func(pt pc.Point) bool {
		v := pt.Position()
		c.Indices[v] = to
		err = c.PointClouds[to].Set(pt)
		c.PointClouds[from].Unset(v.X, v.Y, v.Z)
		return err == nil
	})
	return err
}

// N gives the number of clusters in the partition of the point cloud.
func (c *Clusters) N() int {
	return len(c.PointClouds)
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
