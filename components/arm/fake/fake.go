// Package fake implements a fake arm.
package fake

import (
	"context"
	_ "embed"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Model is the name used to refer to the fake arm model.
var Model = resource.DefaultModelFamily.WithModel("fake")

//go:embed fake_model.json
var fakejson []byte

// Config is used for converting config attributes.
type Config struct {
	ModelFilePath string `json:"model-path,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, []string, error) {
	var err error
	if conf.ModelFilePath == "" {
		_, err = referenceframe.UnmarshalModelJSON(fakejson, "")
	} else {
		_, err = referenceframe.KinematicModelFromFile(conf.ModelFilePath, "")
	}
	return nil, nil, err
}

func init() {
	resource.RegisterComponent(arm.API, Model, resource.Registration[arm.Arm, *Config]{
		Constructor: NewArm,
	})
}

// NewArm returns a new fake arm.
func NewArm(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (arm.Arm, error) {
	a := &Arm{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}
	if err := a.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return a, nil
}

// Arm is a fake arm that can simply read and set properties.
type Arm struct {
	resource.Named
	CloseCount int
	logger     logging.Logger

	mu     sync.RWMutex
	joints []referenceframe.Input
	model  referenceframe.Model
}

// Reconfigure atomically reconfigures this arm in place based on the new config.
func (a *Arm) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	var model referenceframe.Model
	if newConf.ModelFilePath != "" {
		model, err = referenceframe.KinematicModelFromFile(newConf.ModelFilePath, conf.Name)
	} else {
		// if no arm model is specified, we use a fake arm with 1 dof and 0 spatial transformation
		model, err = referenceframe.UnmarshalModelJSON(fakejson, conf.Name)
	}
	if err != nil {
		return err
	}

	dof := len(model.DoF())
	if dof == 0 {
		return errors.New("fake arm built with zero degrees-of-freedom, nothing will show up on the Control tab " +
			"you have either given a kinematics file that resulted in a zero degrees-of-freedom arm or omitted both" +
			"the arm-model and model-path from attributes")
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.joints = referenceframe.FloatsToInputs(make([]float64, dof))
	a.model = model

	return nil
}

// EndPosition returns the set position.
func (a *Arm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	joints, err := a.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return referenceframe.ComputeOOBPosition(a.model, joints)
}

// MoveToPosition sets the position.
func (a *Arm) MoveToPosition(ctx context.Context, pose spatialmath.Pose, extra map[string]interface{}) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	model := a.model
	_, err := model.Transform(a.joints)
	if err != nil && strings.Contains(err.Error(), referenceframe.OOBErrString) {
		return errors.New("cannot move arm: " + err.Error())
	} else if err != nil {
		return err
	}

	plan, err := motionplan.PlanFrameMotion(ctx, a.logger, pose, model, a.joints, nil, nil)
	if err != nil {
		return err
	}
	copy(a.joints, plan[len(plan)-1])
	return nil
}

// MoveToJointPositions sets the joints.
func (a *Arm) MoveToJointPositions(ctx context.Context, joints []referenceframe.Input, extra map[string]interface{}) error {
	if err := arm.CheckDesiredJointPositions(ctx, a, joints); err != nil {
		return err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, err := a.model.Transform(joints)
	if err != nil {
		return err
	}
	copy(a.joints, joints)
	return nil
}

// MoveThroughJointPositions moves the fake arm through the given inputs.
func (a *Arm) MoveThroughJointPositions(
	ctx context.Context,
	positions [][]referenceframe.Input,
	_ *arm.MoveOptions,
	_ map[string]interface{},
) error {
	for _, goal := range positions {
		if err := a.MoveToJointPositions(ctx, goal, nil); err != nil {
			return err
		}
	}
	return nil
}

// JointPositions returns joints.
func (a *Arm) JointPositions(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.joints, nil
}

// Stop doesn't do anything for a fake arm.
func (a *Arm) Stop(ctx context.Context, extra map[string]interface{}) error {
	return nil
}

// IsMoving is always false for a fake arm.
func (a *Arm) IsMoving(ctx context.Context) (bool, error) {
	return false, nil
}

// Kinematics returns the kinematic model supplied for the fake arm.
func (a *Arm) Kinematics(ctx context.Context) (referenceframe.Model, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.model, nil
}

// CurrentInputs returns the current inputs of the fake arm.
func (a *Arm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.joints, nil
}

// GoToInputs moves the fake arm to the given inputs.
func (a *Arm) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	return a.MoveThroughJointPositions(ctx, inputSteps, nil, nil)
}

// Close does nothing.
func (a *Arm) Close(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.CloseCount++
	return nil
}

// Geometries returns the list of geometries associated with the resource, in any order. The poses of the geometries reflect their
// current location relative to the frame of the resource.
func (a *Arm) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	inputs, err := a.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	gif, err := a.model.Geometries(inputs)
	if err != nil {
		return nil, err
	}
	return gif.Geometries(), nil
}
