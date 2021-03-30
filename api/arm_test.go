package api

import (
	"math"
	"testing"

	"go.viam.com/robotcore/utils"

	"github.com/stretchr/testify/assert"
)

func TestArmPosition(t *testing.T) {
	p := NewPositionFromMetersAndRadians(1.0, 2.0, 3.0, 0, math.Pi/2, math.Pi)

	assert.Equal(t, 0.0, p.RX)
	assert.Equal(t, 90.0, p.RY)
	assert.Equal(t, 180.0, p.RZ)

	assert.Equal(t, 0.0, utils.DegToRad(p.RX))
	assert.Equal(t, math.Pi/2, utils.DegToRad(p.RY))
	assert.Equal(t, math.Pi, utils.DegToRad(p.RZ))
}

func TestJointPositions(t *testing.T) {
	in := []float64{0, math.Pi}
	j := JointPositionsFromRadians(in)
	assert.Equal(t, 0.0, j.Degrees[0])
	assert.Equal(t, 180.0, j.Degrees[1])
	assert.Equal(t, in, JointPositionsToRadians(j))
}
