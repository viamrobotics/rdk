package universalrobots

import (
	"io/ioutil"
	"math"
	"testing"

	"go.viam.com/robotcore/artifact"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func Test1(t *testing.T) {
	logger := golog.NewTestLogger(t)
	data, err := ioutil.ReadFile(artifact.MustPath("robots/universalrobots/test1.raw"))
	test.That(t, err, test.ShouldBeNil)

	state, err := readRobotStateMessage(data, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, int(math.Round(state.Joints[0].AngleDegrees())), test.ShouldEqual, 90)
	test.That(t, int(math.Round(state.Joints[1].AngleDegrees())), test.ShouldEqual, -90)
	test.That(t, int(math.Round(state.Joints[2].AngleDegrees())), test.ShouldEqual, 5)
	test.That(t, int(math.Round(state.Joints[3].AngleDegrees())), test.ShouldEqual, 10)
	test.That(t, int(math.Round(state.Joints[4].AngleDegrees())), test.ShouldEqual, 15)
	test.That(t, int(math.Round(state.Joints[5].AngleDegrees())), test.ShouldEqual, 20)
}
