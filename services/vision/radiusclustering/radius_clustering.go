// Package radiusclustering uses the 3D radius clustering algorithm as defined in the
// RDK vision/segmentation package as vision model.
package radiusclustering

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/segmentation"
)

var model = resource.NewDefaultModel("radius_clustering_segmenter")

func init() {
	registry.RegisterService(vision.Subtype, model, registry.Service{
		RobotConstructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			attrs, ok := c.ConvertedAttributes.(*segmentation.RadiusClusteringConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, c.ConvertedAttributes)
			}
			return registerRCSegmenter(ctx, c.Name, attrs, r)
		},
	})
	config.RegisterServiceAttributeMapConverter(
		vision.Subtype,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf segmentation.RadiusClusteringConfig
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*segmentation.RadiusClusteringConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		},
		&segmentation.RadiusClusteringConfig{},
	)
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
		return nil, utils.NewUnexpectedTypeError(segmenter, conf.RadiusClustering)
	}
	return vision.NewService(name, r, nil, nil, nil, segmenter)
}
