package trossen

import (
	"context"
	// for embedding model file.
	_ "embed"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

//go:embed wx250s_kinematics.json
var wx250smodeljson []byte

func init() {
	registry.RegisterComponent(arm.Subtype, "wx250s", registry.Component{
		RobotConstructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewArm(r, config.Attributes, logger, wx250smodeljson)
		},
	})
}
