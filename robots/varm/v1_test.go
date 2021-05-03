package varm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var epsilon = .01

func TestArmMath1(t *testing.T) {
	assert.InDelta(t, 0.0, computeInnerJointAngle(0, 0), epsilon)
	assert.InDelta(t, -90.0, computeInnerJointAngle(0, -90), epsilon)
	assert.InDelta(t, -135.0, computeInnerJointAngle(-45, -90), epsilon)
	assert.InDelta(t, -45.0, computeInnerJointAngle(45, -90), epsilon)

	j := joint{
		posMin: 0.0,
		posMax: 1.0,
		degMin: 0.0,
		degMax: 90.0,
	}

	assert.InDelta(t, 0.0, j.positionToDegrees(0.0), epsilon)
	assert.InDelta(t, 45, j.positionToDegrees(0.5), epsilon)
	assert.InDelta(t, 90, j.positionToDegrees(1.0), epsilon)

	assert.InDelta(t, 0.0, j.degreesToPosition(0.0), epsilon)
	assert.InDelta(t, 0.5, j.degreesToPosition(45.0), epsilon)
	assert.InDelta(t, 1.0, j.degreesToPosition(90.0), epsilon)

}
