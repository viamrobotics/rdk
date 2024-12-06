// Package fake implements a fake arm.
package fake

import (
	"context"
	_ "embed"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/arm"
	ur "go.viam.com/rdk/components/arm/universalrobots"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/referenceframe/urdf"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// errAttrCfgPopulation is the returned error if the Config's fields are fully populated.
var errAttrCfgPopulation = errors.New("can only populate either ArmModel or ModelPath - not both")

// Model is the name used to refer to the fake arm model.
var Model = resource.DefaultModelFamily.WithModel("fake")

var dofbotModel = "yahboom-dofbot"

//go:embed fake_model.json
var fakejson []byte

//go:embed dofbot.json
var dofbotjson []byte

// Config is used for converting config attributes.
type Config struct {
	ArmModel      string `json:"arm-model,omitempty"`
	ModelFilePath string `json:"model-path,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	var err error
	switch {
	case conf.ArmModel != "" && conf.ModelFilePath != "":
		err = errAttrCfgPopulation
	case conf.ArmModel != "" && conf.ModelFilePath == "":
		_, err = modelFromName(conf.ArmModel, "")
	case conf.ArmModel == "" && conf.ModelFilePath != "":
		_, err = modelFromPath(conf.ModelFilePath, "")
	}
	return nil, err
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

func buildModel(cfg resource.Config, newConf *Config) (referenceframe.Model, error) {
	var (
		model referenceframe.Model
		err   error
	)
	armModel := newConf.ArmModel
	modelPath := newConf.ModelFilePath

	switch {
	case armModel != "" && modelPath != "":
		err = errAttrCfgPopulation
	case armModel != "":
		model, err = modelFromName(armModel, cfg.Name)
	case modelPath != "":
		model, err = modelFromPath(modelPath, cfg.Name)
	default:
		// if no arm model is specified, we return a fake arm with 1 dof and 0 spatial transformation
		model, err = modelFromName(Model.Name, cfg.Name)
	}

	return model, err
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

	model, err := buildModel(conf, newConf)
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

// ModelFrame returns the dynamic frame of the model.
func (a *Arm) ModelFrame() referenceframe.Model {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.model
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

func modelFromName(model, name string) (referenceframe.Model, error) {
	switch model {
	case ur.Model.Name:
		return ur.MakeModelFrame(name)
	case dofbotModel:
		return referenceframe.UnmarshalModelJSON(dofbotjson, name)
	case Model.Name:
		return referenceframe.UnmarshalModelJSON(fakejson, name)
	default:
		return nil, errors.Errorf("fake arm cannot be created, unsupported arm-model: %s", model)
	}
}

func modelFromPath(modelPath, name string) (referenceframe.Model, error) {
	switch {
	case strings.HasSuffix(modelPath, ".urdf"):
		return urdf.ParseModelXMLFile(modelPath, name)
	case strings.HasSuffix(modelPath, ".json"):
		return referenceframe.ParseModelJSONFile(modelPath, name)
	default:
		return nil, errors.New("only files with .json and .urdf file extensions are supported")
	}
}
