package api

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArmPosition(t *testing.T) {
	p := NewPositionFromMetersAndRadians(1.0, 2.0, 3.0, 0, math.Pi/2, math.Pi)

	assert.Equal(t, 0.0, p.Rx)
	assert.Equal(t, 90.0, p.Ry)
	assert.Equal(t, 180.0, p.Rz)

	assert.Equal(t, 0.0, p.RxRadians())
	assert.Equal(t, math.Pi/2, p.RyRadians())
	assert.Equal(t, math.Pi, p.RzRadians())
}

func TestJointPositions(t *testing.T) {
	in := []float64{0, math.Pi}
	j := JointPositionsFromRadians(in)
	assert.Equal(t, 0.0, j.Degrees[0])
	assert.Equal(t, 180.0, j.Degrees[1])
	assert.Equal(t, in, j.Radians())
}
