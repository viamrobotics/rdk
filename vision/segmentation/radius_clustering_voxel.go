package segmentation

import (
	"context"
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/vision"
)

// RadiusClusteringVoxelConfig specifies the necessary parameters for 3D object finding.
type RadiusClusteringVoxelConfig struct {
	VoxelSize float64 `json:"voxel_size"`
	Lambda    float64 `json:"lambda"` // clustering parameter for making voxel planes
	*RadiusClusteringConfig
	*VoxelGridPlaneConfig
}

// CheckValid checks to see in the input values are valid.
func (rcc *RadiusClusteringVoxelConfig) CheckValid() error {
	err := rcc.RadiusClusteringConfig.CheckValid()
	if err != nil {
		return err
	}
	err = rcc.VoxelGridPlaneConfig.CheckValid()
	if err != nil {
		return err
	}
	if rcc.VoxelSize <= 0 {
		return errors.Errorf("voxel_size must be greater than 0, got %v", rcc.VoxelSize)
	}
	if rcc.Lambda <= 0 {
		return errors.Errorf("lambda must be greater than 0, got %v", rcc.Lambda)
	}
	return nil
}

// ConvertAttributes changes the AttributeMap input into a RadiusClusteringVoxelConfig.
func (rcc *RadiusClusteringVoxelConfig) ConvertAttributes(am config.AttributeMap) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: rcc})
	if err != nil {
		return err
	}
	clusteringConf := &RadiusClusteringConfig{}
	clusterDecoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: clusteringConf})
	if err != nil {
		return err
	}
	voxelPlaneConf := &VoxelGridPlaneConfig{}
	voxelPlaneDecoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: voxelPlaneConf})
	if err != nil {
		return err
	}
	err = decoder.Decode(am)
	if err != nil {
		return err
	}
	err = clusterDecoder.Decode(am)
	if err != nil {
		return err
	}
	err = voxelPlaneDecoder.Decode(am)
	if err != nil {
		return err
	}
	rcc.RadiusClusteringConfig = clusteringConf
	rcc.VoxelGridPlaneConfig = voxelPlaneConf
	return rcc.CheckValid()
}

// RadiusClusteringFromVoxels removes the planes (if any) and returns a segmentation of the objects in a point cloud.
func RadiusClusteringFromVoxels(ctx context.Context, c camera.Camera, params config.AttributeMap) ([]*vision.Object, error) {
	// convert attributes to appropriate struct
	if params == nil {
		return nil, errors.New("config for radius clustering segmentation cannot be nil")
	}
	cfg := &RadiusClusteringVoxelConfig{}
	err := cfg.ConvertAttributes(params)
	if err != nil {
		return nil, err
	}
	// get next point cloud and convert it to a  VoxelGrid
	// NOTE(bh): Maybe one day cameras will return voxel grids directly.
	cloud, err := c.NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}
	return ApplyRadiusClusteringVoxels(ctx, cloud, cfg)
}

// ApplyRadiusClusteringVoxels turns the cloud into a voxel grid and then does radius clustering  to segment it.
func ApplyRadiusClusteringVoxels(ctx context.Context,
	cloud pc.PointCloud,
	cfg *RadiusClusteringVoxelConfig) ([]*vision.Object, error) {
	// turn the point cloud into a voxel grid
	vg := pc.NewVoxelGridFromPointCloud(cloud, cfg.VoxelSize, cfg.Lambda)
	planeConfig := VoxelGridPlaneConfig{cfg.WeightThresh, cfg.AngleThresh, cfg.CosineThresh, cfg.DistanceThresh}
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
	objConfig := RadiusClusteringConfig{cfg.MinPtsInPlane, cfg.MinPtsInSegment, cfg.ClusteringRadiusMm}
	objVoxGrid := pc.NewVoxelGridFromPointCloud(nonPlane, vg.VoxelSize(), vg.Lambda())
	objects, err := voxelBasedNearestNeighbors(objVoxGrid, objConfig.ClusteringRadiusMm)
	if err != nil {
		return nil, err
	}
	objects = pc.PrunePointClouds(objects, objConfig.MinPtsInSegment)
	segments, err := NewSegmentsFromSlice(objects)
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
