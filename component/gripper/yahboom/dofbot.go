package yahboom

import (
	"context"
	_ "embed" // for embedding model file
	"fmt"

	"github.com/pkg/errors"

	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"

	"github.com/edaniels/golog"
)

func init() {
	registry.RegisterComponent(gripper.Subtype, "yahboom-dofbot", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			armName := config.Attributes.String("arm")
			if armName == "" {
				return nil, errors.New("yahboom-dofbot gripper needs an arm")
			}
			myArm, ok := r.ArmByName(armName)
			if !ok {
				return nil, errors.New("yahboom-dofbot gripper can't find arm")
			}

			goodArm, ok := utils.UnwrapProxy(myArm).(gripper.Gripper)
			if !ok {
				return nil, fmt.Errorf("yahboom-dofbot gripper got not a dofbot arm, got %T", myArm)
			}

			return goodArm, nil
		},
	})

}
