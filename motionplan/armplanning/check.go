package armplanning

import (
	"context"
	"fmt"
	"strings"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// CheckPlanFromRequest checks a plan for collisions by interpolating between each pair of
// consecutive trajectory steps. It is a convenience wrapper around CheckPlan that extracts the
// necessary information from a PlanRequest.
//
// Moving frames are auto-detected by analysing which frames have changing inputs across the
// trajectory. Collisions that exist at the start of the trajectory are automatically allowed
// throughout the plan so that the arm is not penalised for pre-existing contact.
//
// Returns nil if no collision is found, or an error describing the first detected collision.
func CheckPlanFromRequest(
	ctx context.Context,
	logger logging.Logger,
	req *PlanRequest,
	plan motionplan.Plan,
) error {
	ws := req.WorldState
	if ws == nil {
		ws = referenceframe.NewEmptyWorldState()
	}
	return CheckPlan(ctx, logger, ws, req.FrameSystem, plan)
}

// CheckPlan checks a plan for collisions by interpolating between each pair of consecutive
// trajectory steps.
//
// Moving frames are auto-detected by analysing which frames have changing inputs across the
// trajectory. Collisions that exist at the start of the trajectory are automatically allowed
// throughout the plan.
//
// Returns nil if no collision is found, or an error describing the first detected collision.
func CheckPlan(
	ctx context.Context,
	logger logging.Logger,
	worldState *referenceframe.WorldState,
	fs *referenceframe.FrameSystem,
	plan motionplan.Plan,
) error {
	traj := plan.Trajectory()
	if len(traj) < 2 {
		return nil
	}

	// Convert to LinearInputs once for reuse.
	linearTraj := make([]*referenceframe.LinearInputs, len(traj))
	for i, step := range traj {
		linearTraj[i] = step.ToLinearInputs()
	}
	startInputs := linearTraj[0]

	// Get frame geometries at the start configuration.
	frameSystemGeometries, err := referenceframe.FrameSystemGeometriesLinearInputs(fs, startInputs)
	if err != nil {
		return err
	}

	// Auto-detect which frames are actually moving in this trajectory.
	movingFrames := detectMovingFrames(traj)

	// Split geometries into moving and static based on the detected moving frames.
	var movingGeos, staticGeos []spatialmath.Geometry
	for _, geoms := range frameSystemGeometries {
		for _, geom := range geoms.Geometries() {
			if belongsToMovingFrame(geom.Label(), movingFrames) {
				movingGeos = append(movingGeos, geom)
			} else {
				staticGeos = append(staticGeos, geom)
			}
		}
	}

	// Get world obstacle geometries at the start configuration.
	obstaclesInFrame, err := worldState.ObstaclesInWorldFrame(fs, startInputs.ToFrameSystemInputs())
	if err != nil {
		return err
	}
	worldGeos := obstaclesInFrame.Geometries()

	collisionBufferMM := NewBasicPlannerOptions().CollisionBufferMM

	// Determine which collisions already exist at the start so we can ignore them.
	var allowedCollisions []motionplan.Collision
	if len(movingGeos) > 0 {
		if len(worldGeos) > 0 {
			cols, _, err := motionplan.CheckCollisions(movingGeos, worldGeos, nil, collisionBufferMM, true, logger)
			if err != nil {
				return err
			}
			allowedCollisions = append(allowedCollisions, cols...)
		}
		if len(staticGeos) > 0 {
			cols, _, err := motionplan.CheckCollisions(movingGeos, staticGeos, nil, collisionBufferMM, true, logger)
			if err != nil {
				return err
			}
			allowedCollisions = append(allowedCollisions, cols...)
		}
		if len(movingGeos) > 1 {
			cols, _, err := motionplan.CheckCollisions(movingGeos, movingGeos, nil, collisionBufferMM, true, logger)
			if err != nil {
				return err
			}
			allowedCollisions = append(allowedCollisions, cols...)
		}
	}

	collisionConstraints, err := motionplan.CreateAllCollisionConstraints(
		fs, movingGeos, staticGeos, worldGeos, allowedCollisions, collisionBufferMM, nil, logger,
	)
	if err != nil {
		return err
	}

	checker := motionplan.NewEmptyConstraintChecker(logger)
	checker.SetCollisionConstraints(collisionConstraints)

	resolution := defaultResolution
	for i := 0; i < len(linearTraj)-1; i++ {
		seg := &motionplan.SegmentFS{
			StartConfiguration: linearTraj[i],
			EndConfiguration:   linearTraj[i+1],
			FS:                 fs,
		}
		if _, err := checker.CheckStateConstraintsAcrossSegmentFS(ctx, seg, resolution, true); err != nil {
			return fmt.Errorf("collision in segment %d (waypoints %d→%d): %w", i, i, i+1, err)
		}
	}

	return nil
}

// detectMovingFrames returns the set of frame names whose inputs change at any point in the
// trajectory. If no frames change (e.g. all waypoints are identical) every frame in the
// trajectory is treated as moving.
//
// Runs in a single pass, comparing each step to the previous one.
func detectMovingFrames(trajectory motionplan.Trajectory) map[string]bool {
	movingFrames := make(map[string]bool)
	framesInTraj := make(map[string]bool)

	var prev referenceframe.FrameSystemInputs
	first := true
	for _, step := range trajectory {
		for name := range step {
			framesInTraj[name] = true
		}
		if first {
			prev = step
			first = false
			continue
		}
		for name, inputs := range step {
			if movingFrames[name] {
				continue
			}
			prevInputs := prev[name]
			if len(inputs) != len(prevInputs) {
				movingFrames[name] = true
				continue
			}
			for j := range inputs {
				if inputs[j] != prevInputs[j] {
					movingFrames[name] = true
					break
				}
			}
		}
		prev = step
	}

	// Fallback: if nothing moved, treat all frames in the trajectory as moving.
	if len(movingFrames) == 0 {
		for name := range framesInTraj {
			movingFrames[name] = true
		}
	}

	return movingFrames
}

// belongsToMovingFrame returns true when the geometry label starts with one of the moving frame
// names followed by a colon (e.g. "myArm:link1" belongs to frame "myArm").
func belongsToMovingFrame(label string, movingFrames map[string]bool) bool {
	for name := range movingFrames {
		if strings.HasPrefix(label, name+":") {
			return true
		}
	}
	return false
}
