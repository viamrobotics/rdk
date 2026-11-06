package motionplan

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

// TestSharedFramesConstraintSurvivesTopoSetup guards against the shared-frames
// check being registered and then overwritten when the rest of the topological
// constraints are built, which let planners pull rigidly coupled frames apart.
func TestSharedFramesConstraintSurvivesTopoSetup(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	fs := referenceframe.NewEmptyFrameSystem("test")
	for _, name := range []string{"a", "b"} {
		f, err := referenceframe.NewTranslationalFrame(name, r3.Vector{X: 1}, referenceframe.Limit{Min: -1000, Max: 1000})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fs.AddFrame(f, fs.World()), test.ShouldBeNil)
	}
	cfg := func(a, b float64) *referenceframe.LinearInputs {
		return referenceframe.FrameSystemInputs{"a": {a}, "b": {b}}.ToLinearInputs()
	}
	poses := referenceframe.FrameSystemPoses{"a": referenceframe.NewPoseInFrame(referenceframe.World, spatial.NewZeroPose())}

	for _, tc := range []struct {
		name        string
		orientation bool
	}{
		{"shared frames only", false},
		{"alongside an orientation constraint", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			constraints := NewEmptyConstraints()
			constraints.SharedFrames = []SharedFramesConstraint{{
				Frames: []string{"a", "b"}, PositionToleranceMM: 1, OrientationToleranceDegs: 1,
			}}
			if tc.orientation {
				constraints.AddOrientationConstraint(OrientationConstraint{OrientationToleranceDegs: 5})
			}
			c, err := NewConstraintChecker(
				1.0, constraints, poses, poses, fs,
				[]spatial.Geometry{}, []spatial.Geometry{}, nil, cfg(0, 0), nil, logger, nil,
			)
			test.That(t, err, test.ShouldBeNil)

			_, err = c.CheckStateFSConstraints(ctx, &StateFS{Configuration: cfg(10, 10.5), FS: fs})
			test.That(t, err, test.ShouldBeNil)

			_, err = c.CheckStateFSConstraints(ctx, &StateFS{Configuration: cfg(10, 60), FS: fs})
			test.That(t, errors.Is(err, ErrSharedFramesConstraintViolated), test.ShouldBeTrue)
		})
	}
}
