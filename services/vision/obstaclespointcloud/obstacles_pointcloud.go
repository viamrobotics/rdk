// Package obstaclespointcloud uses the 3D radius clustering algorithm as defined in the
// RDK vision/segmentation package as vision model.
package obstaclespointcloud

import (
	"context"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/vision/segmentation"
)

var model = resource.DefaultModelFamily.WithModel("obstacles_pointcloud")

func init() {
	resource.RegisterService(vision.API, model, resource.Registration[vision.Service, *segmentation.ErCCLConfig]{
		Constructor: func(
			ctx context.Context, deps resource.Dependencies, c resource.Config, logger logging.Logger,
		) (vision.Service, error) {
			attrs, err := resource.NativeConfig[*segmentation.ErCCLConfig](c)
			if err != nil {
				return nil, err
			}
			return registerOPSegmenter(ctx, c.ResourceName(), attrs, deps)
		},
		WeakDependencies: []resource.Matcher{
			resource.SubtypeMatcher{Subtype: camera.SubtypeName},
		},
	})
}

// registerOPSegmenter creates a new 3D radius clustering segmenter from the config.
func registerOPSegmenter(
	ctx context.Context,
	name resource.Name,
	conf *segmentation.ErCCLConfig,
	deps resource.Dependencies,
) (vision.Service, error) {
	_, span := trace.StartSpan(ctx, "service::vision::registerObstaclesPointcloud")
	defer span.End()
	if conf == nil {
		return nil, errors.New("config for obstacles pointcloud segmenter cannot be nil")
	}
	err := conf.CheckValid()
	if err != nil {
		return nil, errors.Wrap(err, "obstacles pointcloud segmenter config error")
	}
	segmenter := segmentation.Segmenter(conf.ErCCLAlgorithm)
	return vision.NewService(name, deps, nil, nil, nil, segmenter)
}
