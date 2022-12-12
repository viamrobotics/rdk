package motionplan

import (
	"math"
	"runtime"
)

// default values for inverse kinematics.
const (
	// If an IK solution scores below this much, return it immediately.
	defaultMinIkScore = 0.

	// Number of IK solutions that should be generated before stopping.
	defaultSolutionsToSeed = 50
)

var defaultNumThreads = runtime.NumCPU() / 2

type ikOptions struct {
	constraintHandler
	extra map[string]interface{}

	// Metric by which to measure nearness to the goal
	metric Metric

	// Solutions that score below this amount are considered "good enough" and returned immediately
	MinScore float64 `json:"min_ik_score"`

	// For the below values, if left uninitialized, default values will be used. To disable, set < 0
	// Max number of ik solutions to consider
	MaxSolutions int `json:"max_ik_solutions"`

	// Number of cpu cores to use
	NumThreads int `json:"num_threads"`
}

func newBasicIKOptions() *ikOptions {
	opts := &ikOptions{
		metric:       NewSquaredNormMetric(),
		MinScore:     defaultMinIkScore,
		MaxSolutions: defaultSolutionsToSeed,
		NumThreads:   defaultNumThreads,
	}

	opts.AddConstraint(defaultJointConstraint, NewJointConstraint(math.Inf(1)))
	return opts
}
