package motionplan

import (
	"context"
	"testing"

	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestIKTolerances(t *testing.T) {
	logger := golog.NewTestLogger(t)

	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/varm/v1.json"), "")
	test.That(t, err, test.ShouldBeNil)
	mp, err := NewCBiRRTMotionPlanner(m, logger, nCPU)
	test.That(t, err, test.ShouldBeNil)

	// Test inability to arrive at another position due to orientation
	pos := &pb.Pose{
		X:  -46,
		Y:  0,
		Z:  372,
		OX: -1.78,
		OY: -3.3,
		OZ: -1.11,
	}
	_, err = mp.Plan(context.Background(), pos, frame.FloatsToInputs([]float64{0, 0}))
	test.That(t, err, test.ShouldNotBeNil)

	// Now verify that setting tolerances to zero allows the same arm to reach that position
	mp.SetGradient(PositionOnlyGradient)
	_, err = mp.Plan(context.Background(), pos, frame.FloatsToInputs([]float64{0, 0}))
	test.That(t, err, test.ShouldBeNil)
}
