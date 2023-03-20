package grpc

import (
	"testing"

	"go.viam.com/test"
)

func TestInferSignalingServerAddress(t *testing.T) {
	tests := []struct {
		domain          string
		expectedAddress string
		isSecure        bool
		wasFound        bool
	}{
		{"unknownDomain", "", false, false},
		{"xyz.viam.cloud", "app.viam.com:443", true, true},
		{"abc.xyz.viam.cloud", "app.viam.com:443", true, true},
		{"xyz.robot.viaminternal", "app.viaminternal:8089", true, true},
		{"xyz.viamstg.cloud", "app.viam.dev:443", true, true},
	}

	for _, input := range tests {
		address, secure, ok := InferSignalingServerAddress(input.domain)

		test.That(t, ok, test.ShouldEqual, input.wasFound)
		test.That(t, address, test.ShouldEqual, input.expectedAddress)
		test.That(t, secure, test.ShouldEqual, input.isSecure)
	}
}
