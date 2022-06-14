// Package yahboom implements a yahboom based gripper.
package yahboom

import (
	"context"
	// for embedding model file.
	_ "embed"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(gripper.Subtype, "yahboom-dofbot", registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			armName := config.Attributes.String("arm")
			if armName == "" {
				return nil, errors.New("yahboom-dofbot gripper needs an arm")
			}
			myArm, err := arm.FromDependencies(deps, armName)
			if err != nil {
				return nil, err
			}

			gripper, ok := utils.UnwrapProxy(myArm).(gripper.Gripper)
			if !ok {
				return nil, fmt.Errorf("yahboom-dofbot gripper got not a dofbot arm, got %T", myArm)
			}

			return gripper, nil
		},
	})
}
