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
	m, err := kinematics.ModelFromName(model, name)
	if err != nil {
		return nil, err
	}
	if err := models3d.AttachVisuals(m, model); err != nil {
		return nil, err
	}
	return m, nil
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

// Get3DModels returns the visual meshes the simulated arm's model carries, keyed by link, which
// is empty for an arm model that ships none.
func (sa *simulatedArm) Get3DModels(
	ctx context.Context, extra map[string]interface{},
) (map[string]*commonpb.Mesh, error) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	models := referenceframe.VisualMeshes(sa.model)
	if models == nil {
		models = map[string]*commonpb.Mesh{}
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
