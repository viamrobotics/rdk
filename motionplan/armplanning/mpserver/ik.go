package mpserver

import (
	"context"
	"errors"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
)

// IKInspectCell describes a single IK solution emitted by one nlopt thread, scored and validated the
// same way getSolutions would score and validate it.
type IKInspectCell struct {
	// Cost is the configuration-distance cost of moving from the start configuration to this
	// solution — the same "score" reported in the per-goal solution tables.
	Cost float64
	// GoalDist is the IK solver's own residual: how close the solution's pose is to the goal pose.
	GoalDist float64
	// Exact is true when the solver considered the goal met (GoalDist below the goal threshold).
	Exact bool
	// Inputs is the solution configuration.
	Inputs *referenceframe.LinearInputs

	// Valid is true when the configuration itself passes all state constraints (no self-collision,
	// no obstacle collision, within bounds, ...). When false, StateError explains why.
	Valid      bool
	StateError error

	// CheckPathOK is true when the straight-line interpolation from the start configuration to this
	// solution passes all constraints. Only meaningful when Valid is true. When false, CheckPathError
	// explains why.
	CheckPathOK    bool
	CheckPathError error
}

type IKInspectTable struct {
	Threads [][]IKInspectCell
}

func InspectIK(ctx context.Context, logger logging.Logger, fs *referenceframe.FrameSystem, goal referenceframe.FrameSystemPoses, numSolutions int) (*IKInspectTable, error) {
	return nil, errors.New("unimplemented")
}
