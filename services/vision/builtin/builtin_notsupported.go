//go:build arm || windows

package builtin

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
)

func init() {
	resource.RegisterService(vision.API, resource.DefaultServiceModel, registry.Resource{
		RobotConstructor: func(ctx context.Context, r robot.Robot, c resource.Config, logger golog.Logger) (resource.Resource, error) {
			return nil, errors.New("not supported on 32 bit ARM or Windows")
		},
	})
	resource.RegisterServiceAttributeMapConverter(vision.API, resource.DefaultServiceModel,
		func(attributeMap utils.AttributeMap) (interface{}, error) {
			return nil, errors.New("not supported on 32 bit ARM or Windows")
		},
		&vision.Config{},
	)
}
