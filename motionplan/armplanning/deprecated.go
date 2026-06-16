package armplanning

import (
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
)

// PlanRequestWithWorldState is a deprecated struct that stores all the data necessary to make a
// call to PlanMotion. Though it's deprecated as the `WorldState` is no longer used. The WorldState
// object wraps two concepts, additional geometries and additional transforms. The new PlanRequest
// replaces WorldState with AdditionalGeometries. WorldState transforms are expected to already be
// merged into the `FrameSystem` part of the new PlanRequest.
//
// This type continues to exist as there are existing plan request files that serialized a
// WorldState which we want to continue to be able to parse.
type PlanRequestWithWorldState struct {
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

	myTestOptions testOptions
}

// MustNewWorldState calls NewWorldState and panics if it returns an error.
func mustNewWorldState(obstacles []*referenceframe.GeometriesInFrame, transforms []*referenceframe.LinkInFrame) *referenceframe.WorldState {
	ws, err := referenceframe.NewWorldState(obstacles, transforms)
	if err != nil {
		panic(err)
	}
	return ws
}

// ToPlanRequestWorldStateTransformsIgnored converts a PlanRequestWithWorldState to a PlanRequest by resolving the WorldState
// obstacles into world frame using the embedded FrameSystem and start configuration. WorldState
// transforms are not applied; callers are responsible for merging them into the FrameSystem first.
func (pr *PlanRequestWithWorldState) ToPlanRequestWorldStateTransformsIgnored() (*PlanRequest, error) {
	obstaclesInWorldFrame, err := pr.WorldState.ObstaclesInWorldFrame(
		pr.FrameSystem,
		pr.StartState.Configuration(),
	)
	if err != nil {
		return nil, err
	}

	return &PlanRequest{
		FrameSystem:           pr.FrameSystem,
		Goals:                 pr.Goals,
		StartState:            pr.StartState,
		ObstaclesInWorldFrame: obstaclesInWorldFrame,
		Constraints:           pr.Constraints,
		PlannerOptions:        pr.PlannerOptions,
	}, nil
}
