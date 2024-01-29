package inject

import "go.viam.com/rdk/motionplan"

// Plan is a struct to allow mocking of the motionplan.Plan interface.
type Plan struct {
	PathFunc       func() motionplan.Path
	TrajectoryFunc func() motionplan.Trajectory
}

// Path calls the Plan's mocked PathFunc and returns a Path.
func (p *Plan) Path() motionplan.Path {
	if p.PathFunc == nil {
		return motionplan.Path{}
	}
	return p.PathFunc()
}

// Trajectory calls the Plan's mocked TrajectoryFunc and returns a Trajectory.
func (p *Plan) Trajectory() motionplan.Trajectory {
	if p.TrajectoryFunc == nil {
		return motionplan.Trajectory{}
	}
	return p.TrajectoryFunc()
}
