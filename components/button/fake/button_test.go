package fake_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/button"
	"go.viam.com/rdk/components/button/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func TestPush(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg := resource.Config{
		Name: "fakeButton",
		API:  button.API,
	}
	button, err := fake.NewButton(context.Background(), nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	err = button.Push(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
}
