package inject

import "go.viam.com/rdk/motionplan"

type Plan struct {
	PathFunc       func() motionplan.Path
	TrajectoryFunc func() motionplan.Trajectory
}

func (p *Plan) Path() motionplan.Path {
	if p.PathFunc == nil {
		return motionplan.Path{}
	}
	return p.PathFunc()
}

func (p *Plan) Trajectory() motionplan.Trajectory {
	if p.TrajectoryFunc == nil {
		return motionplan.Trajectory{}
	}
	return p.TrajectoryFunc()
}
