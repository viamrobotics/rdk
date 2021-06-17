package rpcwebrtc

import "testing"

func TestMemoryCallQueue(t *testing.T) {
	testCallQueue(t, NewMemoryCallQueue())
}
