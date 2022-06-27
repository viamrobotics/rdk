package main

import (
	"testing"

	"go.viam.com/test"
)

func TestRun(t *testing.T) {
	_, _, err := RunMotionEstimation()
	test.That(t, err, test.ShouldBeNil)
}
