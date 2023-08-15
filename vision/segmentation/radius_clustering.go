package segmentation

import (
	"context"

	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/camera"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

// RadiusClusteringConfig specifies the necessary parameters to apply the
// radius based clustering algo.
type RadiusClusteringConfig struct {
	resource.TriviallyValidateConfig
	MinPtsInPlane      int       `json:"min_points_in_plane"`
	MaxDistFromPlane   float64   `json:"max_dist_from_plane_mm"`
	NormalVec          r3.Vector `json:"ground_plane_normal_vec"`
	AngleTolerance     float64   `json:"ground_angle_tolerance_degs"`
	MinPtsInSegment    int       `json:"min_points_in_segment"`
	ClusteringRadiusMm float64   `json:"clustering_radius_mm"`
	MeanKFiltering     int       `json:"mean_k_filtering"`
	Label              string    `json:"label,omitempty"`
}

// CheckValid checks to see in the input values are valid.
func (rcc *RadiusClusteringConfig) CheckValid() error {
	if rcc.MinPtsInPlane <= 0 {
		return errors.Errorf("min_points_in_plane must be greater than 0, got %v", rcc.MinPtsInPlane)
	}
	if rcc.MinPtsInSegment <= 0 {
		return errors.Errorf("min_points_in_segment must be greater than 0, got %v", rcc.MinPtsInSegment)
	}
	if rcc.ClusteringRadiusMm <= 0 {
		return errors.Errorf("clustering_radius_mm must be greater than 0, got %v", rcc.ClusteringRadiusMm)
	}
	if rcc.MaxDistFromPlane == 0 {
		rcc.MaxDistFromPlane = 100
	}
	if rcc.MaxDistFromPlane <= 0 {
		return errors.Errorf("max_dist_from_plane must be greater than 0, got %v", rcc.MaxDistFromPlane)
	}
	if rcc.AngleTolerance > 180 || rcc.AngleTolerance < 0 {
		return errors.Errorf("max_angle_of_plane must between 0 & 180 (inclusive), got %v", rcc.AngleTolerance)
	}
	if rcc.NormalVec.Norm2() == 0 {
		rcc.NormalVec = r3.Vector{X: 0, Y: 0, Z: 1}
	}
	if !rcc.NormalVec.IsUnit() {
		return errors.Errorf("ground_plane_normal_vec should be a unit vector, got %v", rcc.NormalVec)
	}
	return nil
}

// ConvertAttributes changes the AttributeMap input into a RadiusClusteringConfig.
func (rcc *RadiusClusteringConfig) ConvertAttributes(am utils.AttributeMap) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: rcc})
	if err != nil {
		return err
	}
	err = decoder.Decode(am)
	if err == nil {
		err = rcc.CheckValid()
	}
	return err
}

// NewRadiusClustering returns a Segmenter that removes the planes (if any) and returns
// a segmentation of the objects in a point cloud using a radius based clustering algo
// described in the paper "A Clustering Method for Efficient Segmentation of 3D Laser Data" by Klasing et al. 2008.
func NewRadiusClustering(params utils.AttributeMap) (Segmenter, error) {
	// convert attributes to appropriate struct
	if params == nil {
		return nil, errors.New("config for radius clustering segmentation cannot be nil")
	}
	cfg := &RadiusClusteringConfig{}
	err := cfg.ConvertAttributes(params)
	if err != nil {
		return nil, err
	}
	return cfg.RadiusClustering, nil
}

// RadiusClustering applies the radius clustering algorithm directly on a given point cloud.
func (rcc *RadiusClusteringConfig) RadiusClustering(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
	// get next point cloud
	cloud, err := src.NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}
	ps := NewPointCloudGroundPlaneSegmentation(cloud, rcc.MaxDistFromPlane, rcc.MinPtsInPlane, rcc.AngleTolerance, rcc.NormalVec)
	// if there are found planes, remove them, and keep all the non-plane points
	_, nonPlane, err := ps.FindGroundPlane(ctx)
	if err != nil {
		return nil, err
	}
	// filter out the noise on the point cloud if mean K is greater than 0
	if rcc.MeanKFiltering > 0.0 {
		filter, err := pc.StatisticalOutlierFilter(rcc.MeanKFiltering, 1.25)
		if err != nil {
			return nil, err
		}
		nonPlane, err = filter(nonPlane)
		if err != nil {
			return nil, err
		}
	}
	// do the segmentation
	segments, err := segmentPointCloudObjects(nonPlane, rcc.ClusteringRadiusMm, rcc.MinPtsInSegment)
	if err != nil {
		return nil, err
	}
	objects, err := NewSegmentsFromSlice(segments, rcc.Label)
	if err != nil {
		return nil, err
	}
	return objects.Objects, nil
}

// segmentPointCloudObjects uses radius based nearest neighbors to segment the images, and then prunes away
// segments that do not pass a certain threshold of points.
func segmentPointCloudObjects(cloud pc.PointCloud, radius float64, nMin int) ([]pc.PointCloud, error) {
	segments, err := radiusBasedNearestNeighbors(cloud, radius)
	if err != nil {
		return nil, err
	}
	segments = pc.PrunePointClouds(segments, nMin)
	return segments, nil
}

// radiusBasedNearestNeighbors partitions the pointcloud, grouping points within a given radius of each other.
func radiusBasedNearestNeighbors(cloud pc.PointCloud, radius float64) ([]pc.PointCloud, error) {
	kdt, ok := cloud.(*pc.KDTree)
	if !ok {
		kdt = pc.ToKDTree(cloud)
	}
	var err error
	clusters := NewSegments()
	c := 0
	kdt.Iterate(0, 0, func(v r3.Vector, d pc.Data) bool {
		// skip if point already is assigned cluster
		if _, ok := clusters.Indices[v]; ok {
			return true
		}
		// if not assigned, see if any of its neighbors are assigned a cluster
		nn := kdt.RadiusNearestNeighbors(v, radius, false)
		for _, neighbor := range nn {
			nv := neighbor.P
			ptIndex, ptOk := clusters.Indices[v]
			neighborIndex, neighborOk := clusters.Indices[nv]
			switch {
			case ptOk && neighborOk:
				if ptIndex != neighborIndex {
					err = clusters.MergeClusters(ptIndex, neighborIndex)
				}
			case !ptOk && neighborOk:
				err = clusters.AssignCluster(v, d, neighborIndex)
			case ptOk && !neighborOk:
				err = clusters.AssignCluster(neighbor.P, neighbor.D, ptIndex)
			}
			if err != nil {
				return false
			}
		}
		// if none of the neighbors were assigned a cluster, create a new cluster and assign all neighbors to it
		if _, ok := clusters.Indices[v]; !ok {
			err = clusters.AssignCluster(v, d, c)
			if err != nil {
				return false
			}
			for _, neighbor := range nn {
				err = clusters.AssignCluster(neighbor.P, neighbor.D, c)
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
