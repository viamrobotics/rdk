package universalrobots

import (
	"io/ioutil"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/stretchr/testify/assert"
)

func Test1(t *testing.T) {
	logger := golog.NewTestLogger(t)
	data, err := ioutil.ReadFile("data/test1.raw")
	if err != nil {
		t.Fatal(err)
	}

	state, err := readRobotStateMessage(data, logger)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 90, int(math.Round(state.Joints[0].AngleDegrees())), "joint 0 is wrong")
	assert.Equal(t, -90, int(math.Round(state.Joints[1].AngleDegrees())), "joint 1 is wrong")
	assert.Equal(t, 5, int(math.Round(state.Joints[2].AngleDegrees())), "joint 2 is wrong")
	assert.Equal(t, 10, int(math.Round(state.Joints[3].AngleDegrees())), "joint 3 is wrong")
	assert.Equal(t, 15, int(math.Round(state.Joints[4].AngleDegrees())), "joint 4 is wrong")
	assert.Equal(t, 20, int(math.Round(state.Joints[5].AngleDegrees())), "joint 5 is wrong")
}
