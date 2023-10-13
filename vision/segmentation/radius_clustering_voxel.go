//go:build !no_media

package segmentation

import (
	"context"
	"fmt"

	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/camera"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

// RadiusClusteringVoxelConfig specifies the necessary parameters for 3D object finding.
type RadiusClusteringVoxelConfig struct {
	VoxelSize          float64 `json:"voxel_size"`
	Lambda             float64 `json:"lambda"` // clustering parameter for making voxel planes
	MinPtsInPlane      int     `json:"min_points_in_plane"`
	MaxDistFromPlane   float64 `json:"max_dist_from_plane"`
	MinPtsInSegment    int     `json:"min_points_in_segment"`
	ClusteringRadiusMm float64 `json:"clustering_radius_mm"`
	WeightThresh       float64 `json:"weight_threshold"`
	AngleThresh        float64 `json:"angle_threshold_degs"`
	CosineThresh       float64 `json:"cosine_threshold"` // between -1 and 1, the value after evaluating Cosine(theta)
	DistanceThresh     float64 `json:"distance_threshold_mm"`
	Label              string  `json:"label,omitempty"`
}

// CheckValid checks to see in the input values are valid.
func (rcc *RadiusClusteringVoxelConfig) CheckValid() error {
	if rcc.VoxelSize <= 0 {
		return errors.Errorf("voxel_size must be greater than 0, got %v", rcc.VoxelSize)
	}
	if rcc.Lambda <= 0 {
		return errors.Errorf("lambda must be greater than 0, got %v", rcc.Lambda)
	}
	radiusClustering := RadiusClusteringConfig{
		MinPtsInPlane:      rcc.MinPtsInPlane,
		MaxDistFromPlane:   rcc.MaxDistFromPlane,
		MinPtsInSegment:    rcc.MinPtsInSegment,
		ClusteringRadiusMm: rcc.ClusteringRadiusMm,
		MeanKFiltering:     50.0,
		Label:              rcc.Label,
	}
	err := radiusClustering.CheckValid()
	if err != nil {
		return err
	}
	voxelPlanes := VoxelGridPlaneConfig{rcc.WeightThresh, rcc.AngleThresh, rcc.CosineThresh, rcc.DistanceThresh}
	err = voxelPlanes.CheckValid()
	if err != nil {
		return err
	}
	return nil
}

// ConvertAttributes changes the AttributeMap input into a RadiusClusteringVoxelConfig.
func (rcc *RadiusClusteringVoxelConfig) ConvertAttributes(am utils.AttributeMap) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: rcc})
	if err != nil {
		return err
	}
	err = decoder.Decode(am)
	if err != nil {
		return err
	}
	return rcc.CheckValid()
}

// NewRadiusClusteringFromVoxels removes the planes (if any) and returns a segmentation of the objects in a point cloud.
func NewRadiusClusteringFromVoxels(params utils.AttributeMap) (Segmenter, error) {
	// convert attributes to appropriate struct
	if params == nil {
		return nil, errors.New("config for radius clustering segmentation cannot be nil")
	}
	cfg := &RadiusClusteringVoxelConfig{}
	err := cfg.ConvertAttributes(params)
	if err != nil {
		return nil, err
	}
	return cfg.RadiusClusteringVoxels, nil
}

// RadiusClusteringVoxels turns the cloud into a voxel grid and then does radius clustering  to segment it.
func (rcc *RadiusClusteringVoxelConfig) RadiusClusteringVoxels(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
	// get next point cloud and convert it to a  VoxelGrid
	// NOTE(bh): Maybe one day cameras will return voxel grids directly.
	cloud, err := src.NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}
	// turn the point cloud into a voxel grid
	vg := pc.NewVoxelGridFromPointCloud(cloud, rcc.VoxelSize, rcc.Lambda)
	planeConfig := VoxelGridPlaneConfig{rcc.WeightThresh, rcc.AngleThresh, rcc.CosineThresh, rcc.DistanceThresh}
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
	objects, err := voxelBasedNearestNeighbors(objVoxGrid, rcc.ClusteringRadiusMm)
	if err != nil {
		return nil, err
	}
	objects = pc.PrunePointClouds(objects, rcc.MinPtsInSegment)
	segments, err := NewSegmentsFromSlice(objects, rcc.Label)
	if err != nil {
		return nil, err
	}

	return segments.Objects, nil
}

func voxelBasedNearestNeighbors(vg *pc.VoxelGrid, radius float64) ([]pc.PointCloud, error) {
	var err error
	vSize := vg.VoxelSize()
	clusters := NewSegments()
	c := 0
	for coord, vox := range vg.Voxels {
		v := r3.Vector{float64(coord.I), float64(coord.J), float64(coord.K)}
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
				for p, d := range vox.Points {
					err = clusters.AssignCluster(p, d, neighborIndex) // label all points in the voxel
					if err != nil {
						return nil, err
					}
				}
			case ptOk && !neighborOk:
				clusters.Indices[nv] = ptIndex
				for p, d := range neighborVox.Points {
					err = clusters.AssignCluster(p, d, ptIndex)
					if err != nil {
						return nil, err
					}
				}
			}
		}
		// if none of the neighbors were assigned a cluster, create a new cluster and assign all neighbors to it
		if _, ok := clusters.Indices[v]; !ok {
			clusters.Indices[v] = c
			for p, d := range vox.Points {
				err = clusters.AssignCluster(p, d, c)
				if err != nil {
					return nil, err
				}
			}
			for nv, neighborVox := range nn {
				clusters.Indices[nv] = c
				for p, d := range neighborVox.Points {
					err = clusters.AssignCluster(p, d, c)
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

func findVoxelNeighborsInRadius(vg *pc.VoxelGrid, coord pc.VoxelCoords, n uint) map[r3.Vector]*pc.Voxel {
	neighbors := make(map[r3.Vector]*pc.Voxel)
	adjCoords := vg.GetNNearestVoxels(vg.Voxels[coord], n)
	for _, c := range adjCoords {
		loc := r3.Vector{float64(c.I), float64(c.J), float64(c.K)}
		neighbors[loc] = vg.Voxels[c]
	}
	return neighbors
}
