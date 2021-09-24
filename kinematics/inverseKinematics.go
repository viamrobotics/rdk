package kinematics

import (
	"context"
	"math"

	"go.viam.com/core/arm"
	pb "go.viam.com/core/proto/api/v1"
	spatial "go.viam.com/core/spatialmath"
)

// goal contains a pose representing a location and orientation to try to reach, and the ID of the end
// effector which should be trying to reach it.
type goal struct {
	GoalTransform spatial.Pose
	EffectorID    int
}

// InverseKinematics TODO
type InverseKinematics interface {
	// Solve receives a context, the goal arm position, and current joint angles.
	// It will return a boolean which will be true if it solved successfully, and the joint positions which
	// will yield that goal position.
	Solve(context.Context, *pb.ArmPosition, *pb.JointPositions) (*pb.JointPositions, error)
	Model() *Model
}

// toArray returns the SolverDistanceWeights as a slice with the components in the same order as the array returned from
// pose ToDelta. Note that orientation components are multiplied by 100 since they are usually small to avoid drift.
func (dc *SolverDistanceWeights) toArray() []float64 {
	return []float64{dc.Trans.X, dc.Trans.Y, dc.Trans.Z, 10 * dc.Orient.TH, 100 * dc.Orient.X, 100 * dc.Orient.Y, 100 * dc.Orient.Z}
}

// SquaredNorm returns the dot product of a vector with itself
func SquaredNorm(vec []float64) float64 {
	norm := 0.0
	for _, v := range vec {
		norm += v * v
	}
	return norm
}

// WeightedSquaredNorm returns the dot product of a vector with itself, applying the given weights to each piece.
func WeightedSquaredNorm(vec []float64, config SolverDistanceWeights) float64 {
	configArr := config.toArray()
	norm := 0.0
	for i, v := range vec {
		norm += v * v * configArr[i]
	}
	return norm
}

// calcSwingPct will calculate the distance from the start position to the halfway point, and also the start position to
// the end position, and return the ratio of the two. If the result >1.0, then the halfway point is further from the
// start position than the end position is, and thus solution searching should continue.
// Positions passed in should be valid, as should their halfway points, so any error will return an infinite distance
func calcSwingPct(from, to *pb.JointPositions, model *Model) (float64, error) {
	startPos, err := model.JointRadToQuat(arm.JointPositionsToRadians(from))
	if err != nil {
		return math.Inf(1), err
	}
	endPos, err := model.JointRadToQuat(arm.JointPositionsToRadians(to))
	if err != nil {
		return math.Inf(1), err
	}
	halfPos, err := model.JointRadToQuat(arm.JointPositionsToRadians(interpolateJoints(from, to, 0.5)))
	if err != nil {
		// This should never happen as one of the above statements should have returned first
		return math.Inf(1), err
	}

	endDist := WeightedSquaredNorm(spatial.PoseDelta(startPos, endPos), model.SolveWeights)
	halfDist := WeightedSquaredNorm(spatial.PoseDelta(startPos, halfPos), model.SolveWeights)
	return (halfDist + 1) / (endDist + 1), nil
}

// bestSolution will select the best solution from a slice of possible solutions for a given model. "Best" is defined
// such that the interpolated halfway point of the motion is most in line with the movement from start to end.
func bestSolution(seedAngles *pb.JointPositions, solutions []*pb.JointPositions, model *Model) (*pb.JointPositions, error) {
	var best *pb.JointPositions
	dist := math.Inf(1)
	for _, solution := range solutions {
		newDist, err := calcSwingPct(seedAngles, solution, model)
		if err != nil {
			return nil, err
		}
		if newDist < dist {
			dist = newDist
			best = solution
		}
	}
	return best, nil
}
