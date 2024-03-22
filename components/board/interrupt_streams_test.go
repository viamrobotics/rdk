package board

import (
	"testing"

	"go.viam.com/test"
)

func TestRemoveStream(t *testing.T) {
	c := &client{}

	stream1 := &interruptStream{
		client: c,
	}
	stream2 := &interruptStream{
		client: c,
	}

	testStreams := []*interruptStream{stream1, stream2}
	c.interruptStreams = testStreams
	expectedStreams := []*interruptStream{stream1}
	c.removeStream(stream2)
	test.That(t, c.interruptStreams, test.ShouldResemble, expectedStreams)
}
