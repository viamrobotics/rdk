package sim

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	commonpb "go.viam.com/api/common/v1"

	models3d "go.viam.com/rdk/components/arm/fake/3d_models"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

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
		return nil, fmt.Errorf("fake arm cannot be created, unsupported arm-model: %s", model)
	}
}

func buildModel(resName string, conf *Config) (referenceframe.Model, error) {
	armModel := conf.Model
	modelPath := conf.ModelFilePath

	switch {
	case armModel != "" && modelPath != "":
		return nil, errors.New("can only populate either Model or ModelPath - not both")
	case armModel != "":
		return modelFromName(armModel, resName)
	case modelPath != "":
		return referenceframe.KinematicModelFromFile(modelPath, resName)
	default:
		return nil, errors.New("a model must be defined for a simulated arm")
	}
}

func (sa *simulatedArm) Get3DModels(
	ctx context.Context, extra map[string]interface{},
) (map[string]*commonpb.Mesh, error) {
	models := make(map[string]*commonpb.Mesh)
	armModelParts := models3d.ArmTo3DModelParts[sa.modelName]
	if armModelParts == nil {
		return models, nil
	}

	for _, modelPart := range armModelParts {
		modelPartMesh := models3d.ThreeDMeshFromName(sa.modelName, modelPart)
		if len(modelPartMesh.Mesh) > 0 {
			// len > 0 indicates we actually have a 3D model for thus armModel and part Name
			models[modelPart] = &modelPartMesh
		} else {
			sa.logger.CWarnw(ctx, "No 3D model found for arm model and part",
				"armModel", sa.modelName, "modelPart", modelPart)
		}
	}

	return models, nil
}

func (sa *simulatedArm) Kinematics(ctx context.Context) (referenceframe.Model, error) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	return sa.model, nil
}

func (sa *simulatedArm) Geometries(
	ctx context.Context, extra map[string]interface{},
) ([]spatialmath.Geometry, error) {
	inputs, err := sa.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}

	gif, err := sa.model.Geometries(inputs)
	if err != nil {
		return nil, err
	}

	return gif.Geometries(), nil
}
