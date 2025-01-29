package fake_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/components/switch/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func TestSwitch(t *testing.T) {
	logger := logging.NewTestLogger(t)
	positionCount := uint32(3)
	cfg := resource.Config{
		Name: "fakeSwitch",
		API:  toggleswitch.API,
		ConvertedAttributes: &fake.Config{
			PositionCount: &positionCount,
		},
	}
	s, err := fake.NewSwitch(context.Background(), nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	n, err := s.GetNumberOfPositions(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, n, test.ShouldEqual, positionCount)
}
