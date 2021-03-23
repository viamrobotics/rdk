package api

import (
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"

	"github.com/stretchr/testify/assert"
)

func TestRegistry(t *testing.T) {
	pf := func(r Robot, config Component, logger golog.Logger) (Provider, error) {
		return nil, nil
	}

	af := func(r Robot, config Component, logger golog.Logger) (Arm, error) {
		return nil, nil
	}

	cf := func(r Robot, config Component, logger golog.Logger) (gostream.ImageSource, error) {
		return nil, nil
	}

	gf := func(r Robot, config Component, logger golog.Logger) (Gripper, error) {
		return nil, nil
	}

	RegisterProvider("x", pf)
	RegisterCamera("x", cf)
	RegisterArm("x", af)
	RegisterGripper("x", gf)

	assert.NotNil(t, ProviderLookup("x"))
	assert.NotNil(t, CameraLookup("x"))
	assert.NotNil(t, ArmLookup("x"))
	assert.NotNil(t, GripperLookup("x"))
}
