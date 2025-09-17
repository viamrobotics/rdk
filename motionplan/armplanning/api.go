// Package armplanning is a motion planning library.
package armplanning

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"

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
	// Set of bounds which the robot must remain within while navigating. This is used only for kinematic bases
	// and not arms.
	BoundingRegions []*commonpb.Geometry `json:"bounding_regions"`
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
	if req.StartState.configuration == nil {
		return errors.New("PlanRequest cannot have nil StartState configuration")
	}
	if req.PlannerOptions == nil {
		req.PlannerOptions = NewBasicPlannerOptions()
	}

	// If we have a start configuration, check for correctness. Reuse FrameSystemPoses compute function to provide error.
	if len(req.StartState.configuration) > 0 {
		_, err := req.StartState.configuration.ComputePoses(req.FrameSystem)
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

	boundingRegions, err := referenceframe.NewGeometriesFromProto(req.BoundingRegions)
	if err != nil {
		return err
	}

	// Validate the goals. Each goal with a pose must not also have a configuration specified. The parent frame of the pose must exist.
	for i, goalState := range req.Goals {
		for fName, pif := range goalState.poses {
			if len(goalState.configuration) > 0 {
				return errors.New("individual goals cannot have both configuration and poses populated")
			}

			goalParentFrame := pif.Parent()
			if req.FrameSystem.Frame(goalParentFrame) == nil {
				return referenceframe.NewParentFrameMissingError(fName, goalParentFrame)
			}

			if len(boundingRegions) > 0 {
				// Check that robot components start within bounding regions.
				// Bounding regions are for 2d planning, which requires a start pose
				if len(goalState.poses) > 0 && len(req.StartState.poses) > 0 {
					goalFrame := req.FrameSystem.Frame(fName)
					if goalFrame == nil {
						return referenceframe.NewFrameMissingError(fName)
					}
					buffer := req.PlannerOptions.CollisionBufferMM
					// check that the request frame's geometries are within or in collision with the bounding regions
					robotGifs, err := goalFrame.Geometries(make([]referenceframe.Input, len(goalFrame.DoF())))
					if err != nil {
						return err
					}
					if i == 0 {
						// Only need to check start poses once
						startPose, ok := req.StartState.poses[fName]
						if !ok {
							return fmt.Errorf("goal frame %s does not have a start pose", fName)
						}
						var robotGeoms []spatialmath.Geometry
						for _, geom := range robotGifs.Geometries() {
							robotGeoms = append(robotGeoms, geom.Transform(startPose.Pose()))
						}
						robotGeomBoundingRegionCheck := motionplan.NewBoundingRegionConstraint(robotGeoms, boundingRegions, buffer)
						if robotGeomBoundingRegionCheck(&motionplan.State{}) != nil {
							return fmt.Errorf("frame named %s is not within the provided bounding regions", fName)
						}
					}

					// check that the destination is within or in collision with the bounding regions
					destinationAsGeom := []spatialmath.Geometry{spatialmath.NewPoint(pif.Pose().Point(), "")}
					destinationBoundingRegionCheck := motionplan.NewBoundingRegionConstraint(destinationAsGeom, boundingRegions, buffer)
					if destinationBoundingRegionCheck(&motionplan.State{}) != nil {
						return errors.New("destination was not within the provided bounding regions")
					}
				}
			}
		}
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
	plan, err := PlanMotion(ctx, logger, &PlanRequest{
		FrameSystem: fs,
		Goals: []*PlanState{
			{poses: referenceframe.FrameSystemPoses{f.Name(): referenceframe.NewPoseInFrame(referenceframe.World, dst)}},
		},
		StartState:     &PlanState{configuration: referenceframe.FrameSystemInputs{f.Name(): seed}},
		Constraints:    constraints,
		PlannerOptions: planOpts,
	})
	if err != nil {
		return nil, err
	}
	return plan.Trajectory().GetFrameInputs(f.Name())
}

// PlanMotion plans a motion from a provided plan request.
func PlanMotion(ctx context.Context, logger logging.Logger, request *PlanRequest) (motionplan.Plan, error) {
	// Make sure request is well formed and not missing vital information
	if err := request.validatePlanRequest(); err != nil {
		return nil, err
	}
	logger.CDebugf(ctx, "constraint specs for this step: %v", request.Constraints)
	logger.CDebugf(ctx, "motion config for this step: %v", request.PlannerOptions)
	logger.CDebugf(ctx, "start position: %v", request.StartState.configuration)

	if request.PlannerOptions == nil {
		request.PlannerOptions = NewBasicPlannerOptions()
	}

	// Theoretically, a plan could be made between two poses, by running IK on both the start and end poses to create sets of seed and
	// goal configurations. However, the blocker here is the lack of a "known good" configuration used to determine which obstacles
	// are allowed to collide with one another.
	if request.StartState.configuration == nil {
		return nil, errors.New("must populate start state configuration")
	}

	sfPlanner, err := newPlanManager(logger, request)
	if err != nil {
		return nil, err
	}

	newPlan, err := sfPlanner.planMultiWaypoint(ctx)
	if err != nil {
		return nil, err
	}

	return newPlan, nil
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
