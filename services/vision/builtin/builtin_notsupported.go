//go:build arm || windows

package builtin

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
)

func init() {
	registry.RegisterService(vision.Subtype, resource.DefaultServiceModel, registry.Service{
		RobotConstructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return nil, errors.New("not supported on 32 bit ARM or Windows")
		},
	})
	config.RegisterServiceAttributeMapConverter(vision.Subtype, resource.DefaultServiceModel,
		func(attributeMap config.AttributeMap) (interface{}, error) {
			return nil, errors.New("not supported on 32 bit ARM or Windows")
		},
		&vision.Attributes{},
	)
}
