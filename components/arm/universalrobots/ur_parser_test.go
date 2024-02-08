package universalrobots

import (
	"context"
	"math"
	"os"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/logging"
)

func Test1(t *testing.T) {
	logger := logging.NewTestLogger(t)
	data, err := os.ReadFile(artifact.MustPath("robots/universalrobots/test1.raw"))
	test.That(t, err, test.ShouldBeNil)

	state, err := readRobotStateMessage(context.Background(), data, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, int(math.Round(state.Joints[0].degrees())), test.ShouldEqual, 90)
	test.That(t, int(math.Round(state.Joints[1].degrees())), test.ShouldEqual, -90)
	test.That(t, int(math.Round(state.Joints[2].degrees())), test.ShouldEqual, 5)
	test.That(t, int(math.Round(state.Joints[3].degrees())), test.ShouldEqual, 10)
	test.That(t, int(math.Round(state.Joints[4].degrees())), test.ShouldEqual, 15)
	test.That(t, int(math.Round(state.Joints[5].degrees())), test.ShouldEqual, 20)
}
