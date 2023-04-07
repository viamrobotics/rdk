// Package radiusclustering uses the 3D radius clustering algorithm as defined in the
// RDK vision/segmentation package as vision model.
package radiusclustering

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/segmentation"
)

var model = resource.NewDefaultModel("radius_clustering_segmenter")

func init() {
	resource.RegisterService(vision.Subtype, model, resource.Registration[vision.Service, *segmentation.RadiusClusteringConfig]{
		DeprecatedRobotConstructor: func(ctx context.Context, r any, c resource.Config, logger golog.Logger) (vision.Service, error) {
			attrs, err := resource.NativeConfig[*segmentation.RadiusClusteringConfig](c)
			if err != nil {
				return nil, err
			}
			actualR, err := utils.AssertType[robot.Robot](r)
			if err != nil {
				return nil, err
			}
			return registerRCSegmenter(ctx, c.ResourceName().Name, attrs, actualR)
		},
	})
}

// registerRCSegmenter creates a new 3D radius clustering segmenter from the config.
func registerRCSegmenter(
	ctx context.Context,
	name string,
	conf *segmentation.RadiusClusteringConfig,
	r robot.Robot,
) (vision.Service, error) {
	_, span := trace.StartSpan(ctx, "service::vision::registerRadiusClustering")
	defer span.End()
	if conf == nil {
		return nil, errors.New("config for radius clustering segmenter cannot be nil")
	}
	err := conf.CheckValid()
	if err != nil {
		return nil, errors.Wrap(err, "radius clustering segmenter config error")
	}
	segmenter := segmentation.Segmenter(conf.RadiusClustering)
	if segmenter == nil {
		return nil, errors.Errorf("type %T is not a segmenter", conf.RadiusClustering)
	}
	return vision.NewService(name, r, nil, nil, nil, segmenter)
}
