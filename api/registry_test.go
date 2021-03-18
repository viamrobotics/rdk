package api

import (
	"testing"

	"github.com/edaniels/gostream"

	"github.com/stretchr/testify/assert"
)

func TestRegistry(t *testing.T) {
	pf := func(r Robot, config Component) (Provider, error) {
		return nil, nil
	}

	af := func(r Robot, config Component) (Arm, error) {
		return nil, nil
	}

	cf := func(r Robot, config Component) (gostream.ImageSource, error) {
		return nil, nil
	}

	gf := func(r Robot, config Component) (Gripper, error) {
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
