package button_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/button"
	"go.viam.com/rdk/components/button/fake"
	"go.viam.com/rdk/resource"
)

const (
	testButtonName    = "button1"
	testButtonName2   = "button2"
	failButtonName    = "button3"
	missingButtonName = "button4"
)

func TestPush(t *testing.T) {
	cfg := resource.Config{
		Name: "fakeButton",
		API:  button.API,
	}
	button, err := fake.NewButton(context.Background(), nil, cfg, nil)
	test.That(t, err, test.ShouldBeNil)

	err = button.Push(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
}
