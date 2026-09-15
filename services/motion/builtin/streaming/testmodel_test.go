package streaming

import (
	"fmt"
	"math"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// testModel builds a dof-joint revolute model whose every joint shares the same
// velocity/acceleration limits (in rad/s and rad/s^2, matching referenceframe.Limit's own
// units), so that referenceframe.TrajectoryLimits(model.DoF()) returns bounded per-joint limits.
// It exists so tests can give an injected arm's KinematicsFunc something for streaming.Run to
// query.
func testModel(dof int, velRadPerSec, accelRadPerSec2 float64) (referenceframe.Model, error) {
	limit := referenceframe.Limit{
		Min:             -math.Pi,
		Max:             math.Pi,
		MaxVelocity:     floatPtr(velRadPerSec),
		MaxAcceleration: floatPtr(accelRadPerSec2),
	}

	fs := referenceframe.NewEmptyFrameSystem("test")
	parent := fs.World()
	var last referenceframe.Frame
	for i := range dof {
		f, err := referenceframe.NewRotationalFrame(fmt.Sprintf("j%d", i), spatialmath.R4AA{RZ: 1}, limit)
		if err != nil {
			return nil, err
		}
		if err := fs.AddFrame(f, parent); err != nil {
			return nil, err
		}
		parent = f
		last = f
	}
	if last == nil {
		return referenceframe.NewSimpleModel("test"), nil
	}
	return referenceframe.NewModel("test", fs, last.Name())
}

func floatPtr(v float64) *float64 {
	return &v
}
