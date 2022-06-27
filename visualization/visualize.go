package visualization

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/rdk/motionplan"
	pb "go.viam.com/rdk/proto/api/common/v1"
	armpb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

type stepData map[string][][]r3.Vector

func VisualizePlan(
	ctx context.Context,
	logger golog.Logger,
	model referenceframe.Model,
	start *armpb.JointPositions,
	goal *pb.Pose,
	worldState *pb.WorldState,
) {
	fs := referenceframe.NewEmptySimpleFrameSystem("test")
	err := fs.AddFrame(model, fs.World())

	from, _ := model.Transform(referenceframe.JointPosToInputs(start))
	to := spatialmath.NewPoseFromProtobuf(goal)

	opt := motionplan.NewDefaultPlannerOptions()
	opt = motionplan.DefaultConstraint(from, to, model, opt)
	opt.RemoveConstraint("self-collision")

	startPos := map[string][]referenceframe.Input{}
	startPos["xArm6"] = referenceframe.JointPosToInputs(start)

	constraint := motionplan.NewCollisionConstraintFromWorldState(model, fs, worldState, startPos)
	opt.AddConstraint("collision", constraint)

	nCPU := runtime.NumCPU()
	mp, err := motionplan.NewCBiRRTMotionPlanner(model, nCPU, logger)
	if err != nil {
		logger.Fatal(err)
	}
	solution, err := mp.Plan(ctx, goal, referenceframe.JointPosToInputs(start), opt)
	if err != nil {
		logger.Fatal(err)
	}
	plan := make([]stepData, 0)
	for _, step := range solution {
		plan = append(plan, getStepData(model, worldState, step))
	}
	visualize(plan)
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
