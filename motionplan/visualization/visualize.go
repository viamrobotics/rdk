// Package visualization provides a minimal way to see from robot's perspective
package visualization

import (
	"context"
	_ "embed"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"

	"github.com/golang/geo/r3"

	pb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
)

//go:embed visualize.py
var vviz []byte

type stepData map[string][][]r3.Vector

// VisualizePlan visualizes a plan for a given model and given worldState.
func VisualizePlan(ctx context.Context, plan [][]referenceframe.Input, model referenceframe.Model, worldState *pb.WorldState) error {
	planData := make([]stepData, 0)
	for _, step := range plan {
		planData = append(planData, getStepData(model, worldState, step))
	}
	return visualize(planData)
}

// VisualizeStep visualizes a single scene including a robot model at given inputs and its world state.
func VisualizeStep(model referenceframe.Frame, worldState *pb.WorldState, inputs []referenceframe.Input) error {
	return visualize([]stepData{getStepData(model, worldState, inputs)})
}

func getStepData(model referenceframe.Frame, worldState *pb.WorldState, inputs []referenceframe.Input) stepData {
	entities := make(map[string][][]r3.Vector)
	if worldState != nil {
		for i, obstacles := range worldState.Obstacles {
			geometries, err := referenceframe.ProtobufToGeometriesInFrame(obstacles)
			if err == nil {
				entities["obstacles"+strconv.Itoa(i)] = getVertices(geometries)
			}
		}
	}
	if model != nil && inputs != nil {
		modelGeometries, err := model.Geometries(inputs)
		if err != nil {
			entities["model"] = getVertices(modelGeometries)
		}
	}
	return entities
}

func visualize(plan []stepData) error {
	// write entities to temporary file
	dataFile, err := ioutil.TempFile("", "vvizdata*.json")
	if err != nil {
		return err
	}
	// nolint:errcheck
	defer os.Remove(dataFile.Name())
	bytes, err := json.MarshalIndent(plan, "", " ")
	if err != nil {
		return err
	}
	if _, err := dataFile.Write(bytes); err != nil {
		return err
	}

	// write script bytes to temporary file
	scriptFile, err := ioutil.TempFile("", "vviz*.py")
	if err != nil {
		return err
	}
	// nolint:errcheck
	defer os.Remove(scriptFile.Name())
	if _, err := scriptFile.Write(vviz); err != nil {
		return err
	}

	// call python visualizer
	// nolint:gosec
	_, err = exec.Command("python3", scriptFile.Name(), dataFile.Name()).Output()
	return err
}

func getVertices(geometries *referenceframe.GeometriesInFrame) [][]r3.Vector {
	vertices := make([][]r3.Vector, 0, len(geometries.Geometries()))
	for _, vol := range geometries.Geometries() {
		vertices = append(vertices, vol.Vertices())
	}
	return vertices
}
