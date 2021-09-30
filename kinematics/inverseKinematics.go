package kinematics

import (
	"context"
	"math"

	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
)

// Motions with swing values less than this are considered good enough to do without looking for better ones
const goodSwingAmt = 1.6

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
	Solve(context.Context, *pb.ArmPosition, []frame.Input) ([]frame.Input, error)
	Model() frame.Frame
	SetSolveWeights(SolverDistanceWeights)
	Close() error
}

// toArray returns the SolverDistanceWeights as a slice with the components in the same order as the array returned from
// pose ToDelta. Note that orientation components are multiplied by 100 since they are usually small to avoid drift.
func (dc *SolverDistanceWeights) toArray() []float64 {
	return []float64{dc.Trans.X, dc.Trans.Y, dc.Trans.Z, 100 * dc.Orient.TH * dc.Orient.X, 100 * dc.Orient.TH * dc.Orient.Y, 100 * dc.Orient.TH * dc.Orient.Z}
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

// calcSwingAmount will calculate the distance from the start position to the halfway point, and also the start position to
// the end position, and return the ratio of the two. If the result >1.0, then the halfway point is further from the
// start position than the end position is, and thus solution searching should continue.
// Positions passed in should be valid, as should their halfway points, so any error will return an infinite distance
func calcSwingAmount(from, to []frame.Input, model frame.Frame) (float64, error) {
	startPos, err := model.Transform(from)
	if err != nil {
		return math.Inf(1), err
	}
	endPos, err := model.Transform(to)
	if err != nil {
		return math.Inf(1), err
	}
	interp := interpolateValues(from, to, 0.5)
	halfPos, err := model.Transform(interp)
	if err != nil {
		// This should never happen as one of the above statements should have returned first
		return math.Inf(1), err
	}

	endDist := SquaredNorm(spatial.PoseDelta(startPos, endPos))
	halfDist := SquaredNorm(spatial.PoseDelta(startPos, halfPos))
	return (halfDist + 1) / (endDist + 1), nil
}

// bestSolution will select the best solution from a slice of possible solutions for a given model. "Best" is defined
// such that the interpolated halfway point of the motion is most in line with the movement from start to end.
func bestSolution(seedAngles []frame.Input, solutions [][]frame.Input, model frame.Frame) ([]frame.Input, float64, error) {
	var best []frame.Input
	dist := math.Inf(1)
	for _, solution := range solutions {
		newDist, err := calcSwingAmount(seedAngles, solution, model)
		if err != nil {
			return nil, math.Inf(1), err
		}
		if newDist < dist {
			dist = newDist
			best = solution
		}
	}
	return best, dist, nil
}

// interpolateValues will return a set of joint positions that are the specified percent between the two given sets of
// positions. For example, setting by to 0.5 will return the joint positions halfway between the inputs, and 0.25 would
// return one quarter of the way from "from" to "to"
func interpolateValues(from, to []frame.Input, by float64) []frame.Input {
	var newVals []frame.Input
	for i, j1 := range from {
		newVals = append(newVals, frame.Input{j1.Value + ((to[i].Value - j1.Value) * by)})
	}
	return newVals
}
