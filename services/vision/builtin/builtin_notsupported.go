//go:build arm

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
        registry.RegisterService(vision.Subtype, resource.DefaultModelName, registry.Service{
                Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
                        return nil, errors.New("not supported on 32 bit arm")
                },
        })
        cType := vision.SubtypeName
        config.RegisterServiceAttributeMapConverter(cType, func(attributeMap config.AttributeMap) (interface{}, error) {
                return nil, errors.New("not supported on 32 bit arm")
        },
                &vision.Attributes{},
        )
}
