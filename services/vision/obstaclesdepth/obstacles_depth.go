//go:build !no_cgo

// Package obstaclesdepth uses an underlying depth camera to fulfill GetObjectPointClouds,
// projecting its depth map to a point cloud, an then applying a point cloud clustering algorithm
package obstaclesdepth

import (
	"context"
	"sort"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/depthadapter"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	svision "go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
	vision "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/segmentation"
)

var model = resource.DefaultModelFamily.WithModel("obstacles_depth")

func init() {
	resource.RegisterService(svision.API, model, resource.Registration[svision.Service, *ObsDepthConfig]{
		DeprecatedRobotConstructor: func(
			ctx context.Context, r any, c resource.Config, logger logging.Logger,
		) (svision.Service, error) {
			attrs, err := resource.NativeConfig[*ObsDepthConfig](c)
			if err != nil {
				return nil, err
			}
			actualR, err := utils.AssertType[robot.Robot](r)
			if err != nil {
				return nil, err
			}
			return registerObstaclesDepth(ctx, c.ResourceName(), attrs, actualR, logging.FromZapCompatible(logger))
		},
	})
}

// ObsDepthConfig specifies the parameters to be used for the obstacle depth service.
type ObsDepthConfig struct {
	resource.TriviallyValidateConfig
	MinPtsInPlane        int     `json:"min_points_in_plane"`
	MinPtsInSegment      int     `json:"min_points_in_segment"`
	MaxDistFromPlane     float64 `json:"max_dist_from_plane_mm"`
	ClusteringRadius     int     `json:"clustering_radius"`
	ClusteringStrictness float64 `json:"clustering_strictness"`
	AngleTolerance       float64 `json:"ground_angle_tolerance_degs"`
}

// obsDepth is the underlying struct actually used by the service.
type obsDepth struct {
	clusteringConf *segmentation.ErCCLConfig
	intrinsics     *transform.PinholeCameraIntrinsics
}

func registerObstaclesDepth(
	ctx context.Context,
	name resource.Name,
	conf *ObsDepthConfig,
	r robot.Robot,
	logger logging.Logger,
) (svision.Service, error) {
	_, span := trace.StartSpan(ctx, "service::vision::registerObstacleDepth")
	defer span.End()
	if conf == nil {
		return nil, errors.New("config for obstacles_depth cannot be nil")
	}
	// build the clustering config
	cfg := &segmentation.ErCCLConfig{
		MinPtsInPlane:        conf.MinPtsInPlane,
		MinPtsInSegment:      conf.MinPtsInSegment,
		MaxDistFromPlane:     conf.MaxDistFromPlane,
		NormalVec:            r3.Vector{0, -1, 0},
		AngleTolerance:       conf.AngleTolerance,
		ClusteringRadius:     conf.ClusteringRadius,
		ClusteringStrictness: conf.ClusteringStrictness,
	}
	err := cfg.CheckValid()
	if err != nil {
		return nil, errors.Wrap(err, "error building clustering config for obstacles_depth")
	}
	myObsDep := &obsDepth{
		clusteringConf: cfg,
	}

	segmenter := myObsDep.buildObsDepth(logger) // does the thing
	return svision.NewService(name, r, nil, nil, nil, segmenter)
}

// BuildObsDepth will check for intrinsics and determine how to build based on that.
func (o *obsDepth) buildObsDepth(logger logging.Logger) func(
	ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
	return func(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
		props, err := src.Properties(ctx)
		if err != nil {
			logger.Warnw("could not find camera properties. obstacles depth started without camera's intrinsic parameters", "error", err)
			return o.obsDepthNoIntrinsics(ctx, src)
		}
		if props.IntrinsicParams == nil {
			logger.Warn("obstacles depth started but camera did not have intrinsic parameters")
			return o.obsDepthNoIntrinsics(ctx, src)
		}
		o.intrinsics = props.IntrinsicParams
		return o.obsDepthWithIntrinsics(ctx, src)
	}
}

// buildObsDepthNoIntrinsics will return the median depth in the depth map as a Geometry point.
func (o *obsDepth) obsDepthNoIntrinsics(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
	pic, release, err := camera.ReadImage(ctx, src)
	if err != nil {
		return nil, errors.Errorf("could not get image from %s", src)
	}
	defer release()

	dm, err := rimage.ConvertImageToDepthMap(ctx, pic)
	if err != nil {
		return nil, errors.New("could not convert image to depth map")
	}
	depData := dm.Data()
	if len(depData) == 0 {
		return nil, errors.New("could not get info from depth map")
	}
	// Sort the depth data [smallest...largest]
	sort.Slice(depData, func(i, j int) bool {
		return depData[i] < depData[j]
	})
	med := int(0.5 * float64(len(depData)))
	pt := spatialmath.NewPoint(r3.Vector{X: 0, Y: 0, Z: float64(depData[med])}, "")
	toReturn := make([]*vision.Object, 1)
	toReturn[0] = &vision.Object{Geometry: pt}
	return toReturn, nil
}

// buildObsDepthWithIntrinsics will use the methodology in Manduchi et al. to find obstacle points
// before clustering and projecting those points into 3D obstacles.
func (o *obsDepth) obsDepthWithIntrinsics(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
	// Check if we have intrinsics here. If not, don't even try
	if o.intrinsics == nil {
		return nil, errors.New("tried to build obstacles depth with intrinsics but no instrinsics found")
	}
	pic, release, err := camera.ReadImage(ctx, src)
	if err != nil {
		return nil, errors.Errorf("could not get image from %s", src)
	}
	defer release()
	dm, err := rimage.ConvertImageToDepthMap(ctx, pic)
	if err != nil {
		return nil, errors.New("could not convert image to depth map")
	}
	cloud := depthadapter.ToPointCloud(dm, o.intrinsics)
	return segmentation.ApplyERCCLToPointCloud(ctx, cloud, o.clusteringConf)
}
