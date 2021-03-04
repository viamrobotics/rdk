package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRolling1(t *testing.T) {
	ra := NewRollingAverage(2)
	ra.Add(5)
	ra.Add(9)
	assert.Equal(t, 7, ra.Average())

	ra.Add(11)
	assert.Equal(t, 10, ra.Average())
}
