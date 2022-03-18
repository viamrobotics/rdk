package segmentation

import (
	"context"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

// RadiusClusteringSegmenter is  the name of a segmenter that finds well separated objects on a flat plane.
const RadiusClusteringSegmenter = "radius_clustering"

func init() {
	RegisterSegmenter(RadiusClusteringSegmenter,
		Registration{
			Segmenter(RadiusClustering),
			utils.JSONTags(RadiusClusteringConfig{}),
		})
}

// RadiusClusteringConfig specifies the necessary parameters for 3D object finding.
type RadiusClusteringConfig struct {
	MinPtsInPlane      int     `json:"min_points_in_plane"`
	MinPtsInSegment    int     `json:"min_points_in_segment"`
	ClusteringRadiusMm float64 `json:"clustering_radius_mm"`
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
	return nil
}

// ConvertAttributes changes the AttributeMap input into a RadiusClusteringConfig.
func (rcc *RadiusClusteringConfig) ConvertAttributes(am config.AttributeMap) error {
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

// RadiusClustering is a Segmenter that removes the planes (if any) and returns
// a segmentation of the objects in a point cloud using a radius based clustering algo
// described in the paper "A Clustering Method for Efficient Segmentation of 3D Laser Data" by Klasing et al. 2008.
func RadiusClustering(ctx context.Context, c camera.Camera, params config.AttributeMap) ([]*vision.Object, error) {
	// convert attributes to appropriate struct
	if params == nil {
		return nil, errors.New("config for radius clustering segmentation cannot be nil")
	}
	cfg := &RadiusClusteringConfig{}
	err := cfg.ConvertAttributes(params)
	if err != nil {
		return nil, err
	}
	// get next point cloud
	cloud, err := c.NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}
	return RadiusClusteringOnPointCloud(ctx, cloud, cfg)
}

// RadiusClusteringOnPointCloud applies the radius clustering algorithm directly on a given point cloud.
func RadiusClusteringOnPointCloud(ctx context.Context, cloud pc.PointCloud, cfg *RadiusClusteringConfig) ([]*vision.Object, error) {
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
