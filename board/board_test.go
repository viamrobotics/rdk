package board

import (
	"testing"

	pb "go.viam.com/robotcore/proto/api/v1"

	"github.com/stretchr/testify/assert"
)

func TestFlipDirection(t *testing.T) {
	assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED, FlipDirection(pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED))
	assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, FlipDirection(pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD))
	assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, FlipDirection(pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD))
}
