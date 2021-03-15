package board

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirectionFromString(t *testing.T) {
	assert.Equal(t, DirNone, DirectionFromString(""))
	assert.Equal(t, DirNone, DirectionFromString("x"))

	assert.Equal(t, DirForward, DirectionFromString("f"))
	assert.Equal(t, DirForward, DirectionFromString("for"))

	assert.Equal(t, DirBackward, DirectionFromString("b"))
	assert.Equal(t, DirBackward, DirectionFromString("back"))
}
