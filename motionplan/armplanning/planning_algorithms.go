package armplanning

// AlgorithmSettings is a polymorphic representation of motion planning algorithms and their parameters. The `Algorithm`
// should correlate with the available options (e.g. if `Algorithm` us CBiRRT, RRTStarOpts should be nil and CBirrtOpts should not).
type AlgorithmSettings struct {
	CBirrtOpts *cbirrtOptions `json:"cbirrt_settings"`
}

// move back to cBiRRT.go when motionplan is taken out of RDK.
type cbirrtOptions struct {
	// Number of IK solutions with which to seed the goal side of the bidirectional tree.
	SolutionsToSeed int `json:"solutions_to_seed"`

	// This is how far cbirrt will try to extend the map towards a goal per-step. Determined from FrameStep
	qstep map[string][]float64
}
