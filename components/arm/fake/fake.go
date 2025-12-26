// Package fake implements a fake arm.
package fake

import (
	"context"
	_ "embed"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/components/arm"
	models3d "go.viam.com/rdk/components/arm/fake/3d_models"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// errAttrCfgPopulation is the returned error if the Config's fields are fully populated.
var errAttrCfgPopulation = errors.New("can only populate either ArmModel or ModelPath - not both")

// Model is the name used to refer to the fake arm model.
var Model = resource.DefaultModelFamily.WithModel("fake")

// Config is used for converting config attributes.
type Config struct {
	ArmModel      string `json:"arm-model,omitempty"`
	ModelFilePath string `json:"model-path,omitempty"`
}

// Known values that can be provided for the ArmModel field.
var (
	ur5eModel  = "ur5e"
	ur20Model  = "ur20"
	xArm6Model = "xarm6"
	xArm7Model = "xarm7"
	lite6Model = "lite6"
)

//go:embed kinematics/fake.json
var fakejson []byte

//go:embed kinematics/ur5e.json
var ur5eJSON []byte

//go:embed kinematics/ur20.json
var ur20JSON []byte

//go:embed kinematics/xarm6.json
var xarm6JSON []byte

//go:embed kinematics/xarm7.json
var xarm7JSON []byte

//go:embed kinematics/lite6.json
var lite6JSON []byte

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, []string, error) {
	var err error
	switch {
	case conf.ArmModel != "" && conf.ModelFilePath != "":
		err = errAttrCfgPopulation
	case conf.ArmModel != "" && conf.ModelFilePath == "":
		_, err = modelFromName(conf.ArmModel, "")
	case conf.ArmModel == "" && conf.ModelFilePath != "":
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
		model, err = referenceframe.KinematicModelFromFile(modelPath, cfg.Name)
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

	// Writes to `joints` or `model` must hold the write-lock. And reads to `joints` or `model` must
	// hold the read-lock.
	mu       sync.RWMutex
	joints   []referenceframe.Input
	model    referenceframe.Model
	armModel string
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
	a.joints = make([]referenceframe.Input, dof)
	a.model = model
	a.armModel = newConf.ArmModel
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
	a.mu.Lock()
	defer a.mu.Unlock()

	model := a.model
	_, err := model.Transform(a.joints)
	if err != nil && strings.Contains(err.Error(), referenceframe.OOBErrString) {
		return errors.New("cannot move arm: " + err.Error())
	} else if err != nil {
		return err
	}

	plan, err := motionplan.GetGlobal().PlanFrameMotion(ctx, a.logger, pose, model, a.joints, nil, nil)
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
	a.mu.Lock()
	defer a.mu.Unlock()
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

	ret := make([]referenceframe.Input, len(a.joints))
	copy(ret, a.joints)
	return ret, nil
}

func (a *Arm) StreamJointPositions(ctx context.Context, fps int32, extra map[string]interface{}) (chan *arm.JointPositionsStreamed, error) {
	if fps <= 0 {
		fps = 30
	}

	ch := make(chan *arm.JointPositionsStreamed, 8)
	ticker := time.NewTicker(time.Second / time.Duration(fps))

	go func() {
		defer close(ch)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				positions, err := a.JointPositions(ctx, extra)
				if err != nil {
					return
				}

				ch <- &arm.JointPositionsStreamed{
					Positions: positions,
					Timestamp: time.Now(),
				}
			}
		}
	}()

	return ch, nil
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

	ret := make([]referenceframe.Input, len(a.joints))
	copy(ret, a.joints)

	return ret, nil
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
	a.mu.RLock()
	defer a.mu.RUnlock()
	gif, err := a.model.Geometries(inputs)
	if err != nil {
		return nil, err
	}
	return gif.Geometries(), nil
}

// Get3DModels returns the 3D models of the fake arm. Unknown arm models should return an empty map.
func (a *Arm) Get3DModels(ctx context.Context, extra map[string]interface{}) (map[string]*commonpb.Mesh, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	models := make(map[string]*commonpb.Mesh)
	armModelParts := models3d.ArmTo3DModelParts[a.armModel]
	if armModelParts == nil {
		return models, nil
	}

	for _, modelPart := range armModelParts {
		modelPartMesh := models3d.ThreeDMeshFromName(a.armModel, modelPart)
		if len(modelPartMesh.Mesh) > 0 {
			// len > 0 indicates we actually have a 3D model for thus armModel and part Name
			models[modelPart] = &modelPartMesh
		} else {
			a.logger.CWarnw(ctx, "No 3D model found for arm model and part", "armModel", a.armModel, "modelPart", modelPart)
		}
	}

	return models, nil
}

func modelFromName(model, name string) (referenceframe.Model, error) {
	switch model {
	case ur5eModel:
		return referenceframe.UnmarshalModelJSON(ur5eJSON, name)
	case ur20Model:
		return referenceframe.UnmarshalModelJSON(ur20JSON, name)
	case xArm6Model:
		return referenceframe.UnmarshalModelJSON(xarm6JSON, name)
	case xArm7Model:
		return referenceframe.UnmarshalModelJSON(xarm7JSON, name)
	case lite6Model:
		return referenceframe.UnmarshalModelJSON(lite6JSON, name)
	case Model.Name:
		return referenceframe.UnmarshalModelJSON(fakejson, name)
	default:
		return nil, errors.Errorf("fake arm cannot be created, unsupported arm-model: %s", model)
	}
}
