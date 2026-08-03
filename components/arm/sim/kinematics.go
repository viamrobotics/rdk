package sim

import (
	"context"
	"errors"

	commonpb "go.viam.com/api/common/v1"

	models3d "go.viam.com/rdk/components/arm/fake/3d_models"
	"go.viam.com/rdk/components/arm/kinematics"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

func modelFromName(model, name string) (referenceframe.Model, error) {
	if model == Model.Name {
		// The simulated arm's own model name selects the generic kinematics.
		model = kinematics.Fake
	}
	return kinematics.ModelFromName(model, name)
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
