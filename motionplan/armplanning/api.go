// Package armplanning is a motion planning library.
package armplanning

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// PlanRequest is a struct to store all the data necessary to make a call to PlanMotion.
type PlanRequest struct {
	FrameSystem *referenceframe.FrameSystem `json:"frame_system"`

	// The planner will hit each Goal in order. Each goal may be a configuration or FrameSystemPoses for holonomic motion, or must be a
	// FrameSystemPoses for non-holonomic motion. For holonomic motion, if both a configuration and FrameSystemPoses are given,
	// an error is thrown.
	// TODO: Perhaps we could do something where some components are enforced to arrive at a certain configuration, but others can have IK
	// run to solve for poses. Doing this while enforcing configurations may be tricky.
	Goals []*PlanState `json:"goals"`

	// This must always have a configuration filled in, for geometry placement purposes.
	// If poses are also filled in, the configuration will be used to determine geometry collisions, but the poses will be used
	// in IK to generate plan start configurations. The given configuration will NOT automatically be added to the seed tree.
	// The use case here is that if a particularly difficult path must be planned between two poses, that can be done first to ensure
	// feasibility, and then other plans can be requested to connect to that returned plan's configurations.
	StartState *PlanState `json:"start_state"`
	// The data representation of the robot's environment.
	WorldState *referenceframe.WorldState `json:"world_state"`
	// Additional parameters constraining the motion of the robot.
	Constraints *motionplan.Constraints `json:"constraints"`
	// Other more granular parameters for the plan used to move the robot.
	PlannerOptions *PlannerOptions `json:"planner_options"`
}

// validatePlanRequest ensures PlanRequests are not malformed.
func (req *PlanRequest) validatePlanRequest() error {
	if req == nil {
		return errors.New("PlanRequest cannot be nil")
	}
	if req.FrameSystem == nil {
		return errors.New("PlanRequest cannot have nil framesystem")
	}

	if req.StartState == nil {
		return errors.New("PlanRequest cannot have nil StartState")
	}
	if req.StartState.structuredConfiguration == nil {
		return errors.New("PlanRequest cannot have nil StartState configuration")
	}
	if req.PlannerOptions == nil {
		req.PlannerOptions = NewBasicPlannerOptions()
	}

	// If we have a start configuration, check for correctness. Reuse FrameSystemPoses compute function to provide error.
	if len(req.StartState.structuredConfiguration) > 0 {
		_, err := req.StartState.structuredConfiguration.ComputePoses(req.FrameSystem)
		if err != nil {
			return err
		}
	}

	// if we have start poses, check we have valid frames
	for fName, pif := range req.StartState.poses {
		if req.FrameSystem.Frame(fName) == nil {
			return referenceframe.NewFrameMissingError(fName)
		}
		if req.FrameSystem.Frame(pif.Parent()) == nil {
			return referenceframe.NewParentFrameMissingError(fName, pif.Parent())
		}
	}

	if len(req.Goals) == 0 {
		return errors.New("PlanRequest must have at least one goal")
	}

	if req.PlannerOptions.MeshesAsOctrees {
		// convert any meshes in the worldstate to octrees
		if req.WorldState == nil {
			return errors.New("PlanRequest must have non-nil WorldState if 'meshes_as_octrees' option is enabled")
		}
		obstacles := make([]*referenceframe.GeometriesInFrame, 0, len(req.WorldState.ObstacleNames()))
		for _, gf := range req.WorldState.Obstacles() {
			geometries := gf.Geometries()
			pcdGeometries := make([]spatialmath.Geometry, 0, len(geometries))
			for _, geometry := range geometries {
				if mesh, ok := geometry.(*spatialmath.Mesh); ok {
					octree, err := pointcloud.NewFromMesh(mesh)
					if err != nil {
						return err
					}
					geometry = octree
				}
				pcdGeometries = append(pcdGeometries, geometry)
			}
			obstacles = append(obstacles, referenceframe.NewGeometriesInFrame(gf.Parent(), pcdGeometries))
		}
		newWS, err := referenceframe.NewWorldState(obstacles, req.WorldState.Transforms())
		if err != nil {
			return err
		}
		req.WorldState = newWS
	}

	// Validate the goals. Each goal with a pose must not also have a configuration specified. The parent frame of the pose must exist.
	for _, goalState := range req.Goals {
		for fName, pif := range goalState.poses {
			if len(goalState.structuredConfiguration) > 0 {
				return errors.New("individual goals cannot have both configuration and poses populated")
			}

			goalParentFrame := pif.Parent()
			if req.FrameSystem.Frame(goalParentFrame) == nil {
				return referenceframe.NewParentFrameMissingError(fName, goalParentFrame)
			}
		}
	}

	if req.Constraints == nil {
		req.Constraints = &motionplan.Constraints{}
	}
	return nil
}

// PlanFrameMotion plans a motion to destination for a given frame with no frame system. It will create a new FS just for the plan.
// WorldState is not supported in the absence of a real frame system.
func PlanFrameMotion(ctx context.Context,
	logger logging.Logger,
	dst spatialmath.Pose,
	f referenceframe.Frame,
	seed []referenceframe.Input,
	constraints *motionplan.Constraints,
	planningOpts map[string]interface{},
) ([][]referenceframe.Input, error) {
	// ephemerally create a framesystem containing just the frame for the solve
	fs := referenceframe.NewEmptyFrameSystem("")
	if err := fs.AddFrame(f, fs.World()); err != nil {
		return nil, err
	}
	planOpts, err := NewPlannerOptionsFromExtra(planningOpts)
	if err != nil {
		return nil, err
	}
	plan, _, err := PlanMotion(ctx, logger, &PlanRequest{
		FrameSystem: fs,
		Goals: []*PlanState{
			{poses: referenceframe.FrameSystemPoses{f.Name(): referenceframe.NewPoseInFrame(referenceframe.World, dst)}},
		},
		StartState:     &PlanState{structuredConfiguration: referenceframe.FrameSystemInputs{f.Name(): seed}},
		Constraints:    constraints,
		PlannerOptions: planOpts,
	})
	if err != nil {
		return nil, err
	}
	return plan.Trajectory().GetFrameInputs(f.Name())
}

// PlanMeta is meta data about plan generation.
type PlanMeta struct {
	Duration       time.Duration
	Partial        bool
	GoalsProcessed int
}

// PlanMotion plans a motion from a provided plan request.
func PlanMotion(ctx context.Context, logger logging.Logger, request *PlanRequest) (motionplan.Plan, *PlanMeta, error) {
	start := time.Now()
	meta := &PlanMeta{}
	ctx, span := trace.StartSpan(ctx, "PlanMotion")
	defer func() {
		meta.Duration = time.Since(start)
		span.End()
	}()

	if err := request.validatePlanRequest(); err != nil {
		return nil, meta, err
	}
	logger.CDebugf(ctx, "constraint specs for this step: %v", request.Constraints)
	logger.CDebugf(ctx, "motion config for this step: %v", request.PlannerOptions)
	logger.CDebugf(ctx, "start position: %v", request.StartState.structuredConfiguration)

	if request.PlannerOptions == nil {
		request.PlannerOptions = NewBasicPlannerOptions()
	}

	// Theoretically, a plan could be made between two poses, by running IK on both the start and end poses to create sets of seed and
	// goal configurations. However, the blocker here is the lack of a "known good" configuration used to determine which obstacles
	// are allowed to collide with one another.
	if request.StartState.structuredConfiguration == nil {
		return nil, meta, errors.New("must populate start state configuration")
	}

	sfPlanner, err := newPlanManager(ctx, logger, request, meta)
	if err != nil {
		return nil, meta, err
	}

	trajAsInps, goalsProcessed, err := sfPlanner.planMultiWaypoint(ctx)
	if err != nil {
		if request.PlannerOptions.ReturnPartialPlan {
			meta.Partial = true
			logger.Infof("returning partial plan, error: %v", err)
		} else {
			return nil, meta, err
		}
	}

	meta.GoalsProcessed = goalsProcessed

	t, err := motionplan.NewSimplePlanFromTrajectory(trajAsInps, request.FrameSystem)
	if err != nil {
		return nil, meta, err
	}

	return t, meta, nil
}

var defaultArmPlannerOptions = &motionplan.Constraints{
	LinearConstraint: []motionplan.LinearConstraint{},
}

// MoveArm is a helper function to abstract away movement for general arms.
func MoveArm(ctx context.Context, logger logging.Logger, a arm.Arm, dst spatialmath.Pose) error {
	inputs, err := a.CurrentInputs(ctx)
	if err != nil {
		return err
	}

	model, err := a.Kinematics(ctx)
	if err != nil {
		return err
	}
	_, err = model.Transform(inputs)
	if err != nil && strings.Contains(err.Error(), referenceframe.OOBErrString) {
		return errors.New("cannot move arm: " + err.Error())
	} else if err != nil {
		return err
	}

	plan, err := PlanFrameMotion(ctx, logger, dst, model, inputs, defaultArmPlannerOptions, nil)
	if err != nil {
		return err
	}
	return a.MoveThroughJointPositions(ctx, plan, nil, nil)
}

// ReadRequestFromFile reads a PlanRequest from a json file.
func ReadRequestFromFile(fileName string) (*PlanRequest, error) {
	f, err := os.Open(fileName) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(f.Close)

	decoder := json.NewDecoder(f)

	req := &PlanRequest{}

	err = decoder.Decode(req)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// WriteToFile write a request to a .json file.
func (req *PlanRequest) WriteToFile(fileName string) error {
	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Clean(fileName), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(file.Close)
	_, err = file.Write(data)
	if err != nil {
		return err
	}
	return nil
}
