package wheeled

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

type kinematicWheeledBase struct {
	*wheeledBase
	slam  slam.Service
	model referenceframe.Model
}

// WrapWithKinematics takes a wheeledBase component and adds a slam service to it
// It also adds kinematic model so that it can be controlled.
func (base *wheeledBase) WrapWithKinematics(ctx context.Context, slamSvc slam.Service) (base.KinematicBase, error) {
	var err error
	wb, ok := utils.UnwrapProxy(base).(*wheeledBase)
	if !ok {
		return nil, errors.Errorf("could not interpret base of type %T as a wheeledBase", base)
	}
	kwb := &kinematicWheeledBase{
		wheeledBase: wb,
		slam:        slamSvc,
	}
	limits, err := slam.Limits(ctx, slamSvc)
	if err != nil {
		return nil, err
	}
	kwb.model, err = Model(kwb.name, kwb.collisionGeometry, limits)
	if err != nil {
		return nil, err
	}
	return kwb, err
}

func (kwb *kinematicWheeledBase) ModelFrame() referenceframe.Model {
	return kwb.model
}

func (kwb *kinematicWheeledBase) currentPose(ctx context.Context) (spatialmath.Pose, error) {
	// TODO: make a transformation from the component reference to the base frame
	pose, _, err := kwb.slam.GetPosition(ctx)
	return pose, err
}

func (kwb *kinematicWheeledBase) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	// TODO(RSDK-2311): complete the implementation
	return []referenceframe.Input{}, errors.New("not implemented yet")
}

func (kwb *kinematicWheeledBase) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	// TODO(RSDK-2311): complete the implementation
	return errors.New("not implemented yet")
}

// ModelFrame builds the kinematic model associated with the kinematicWheeledBase
// Note that this model is not intended to be registered in the frame system.
func Model(name string, collisionGeometry spatialmath.Geometry, limits []referenceframe.Limit) (referenceframe.Model, error) {
	// build the model - SLAM convention is that the XZ plane is the ground plane
	frame2D, err := referenceframe.NewMobile2DFrame(collisionGeometry.Label(), limits, collisionGeometry)
	if err != nil {
		return nil, err
	}
	model := referenceframe.NewSimpleModel(name)
	model.OrdTransforms = []referenceframe.Frame{frame2D}
	return model, nil
}
