package visualization

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"os/exec"
	"strconv"

	"github.com/golang/geo/r3"
	pb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

type stepData map[string][][]r3.Vector

func VisualizePlan(ctx context.Context, plan [][]referenceframe.Input, model referenceframe.Model, worldState *pb.WorldState) {
	planData := make([]stepData, 0)
	for _, step := range plan {
		planData = append(planData, getStepData(model, worldState, step))
	}
	visualize(planData)
}

func VisualizeStep(model referenceframe.Model, worldState *pb.WorldState, inputs []referenceframe.Input) {
	visualize([]stepData{getStepData(model, worldState, inputs)})
}

func getStepData(model referenceframe.Model, worldState *pb.WorldState, inputs []referenceframe.Input) stepData {
	entities := make(map[string][][]r3.Vector)
	if worldState != nil {
		for i, obstacles := range worldState.Obstacles {
			geometries, err := referenceframe.ProtobufToGeometriesInFrame(obstacles)
			if err != nil {
				log.Fatal("No geometries to write")
			}
			entities["obstacles"+strconv.Itoa(i)] = getVertices(geometries)
		}
	}
	if model != nil && inputs != nil {
		modelGeometries, _ := model.Geometries(inputs)
		entities["model"] = getVertices(modelGeometries)
	}
	return entities
}

func visualize(plan []stepData) {
	// write entities to temporary file
	tempFile := utils.ResolveFile("visualization/temp.json")
	bytes, err := json.MarshalIndent(plan, "", " ")
	if err != nil {
		log.Fatal("Could not marshal JSON")
	}
	if err := ioutil.WriteFile(tempFile, bytes, 0644); err != nil {
		log.Fatal("Could not write JSON to file")
	}

	// call python visualizer
	_, err = exec.Command("python3", utils.ResolveFile("visualization/visualize.py"), tempFile).Output()
	if err != nil {
		log.Fatal(err.Error())
	}

	// clean up
	_, err = exec.Command("rm", tempFile).Output()
	if err != nil {
		log.Fatal(err.Error())
	}
}

func getVertices(geometries *referenceframe.GeometriesInFrame) [][]r3.Vector {
	if geometries == nil {
		log.Fatal("No geometries to write")
	}

	vertices := make([][]r3.Vector, 0, len(geometries.Geometries()))
	for _, vol := range geometries.Geometries() {
		vertices = append(vertices, vol.Vertices())
	}
	return vertices
}
