package api

import (
	"fmt"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
)

type CreateProvider func(r Robot, config Component, logger golog.Logger) (Provider, error)
type CreateCamera func(r Robot, config Component, logger golog.Logger) (gostream.ImageSource, error)
type CreateArm func(r Robot, config Component, logger golog.Logger) (Arm, error)
type CreateGripper func(r Robot, config Component, logger golog.Logger) (Gripper, error)
type CreateBase func(r Robot, config Component, logger golog.Logger) (Base, error)

var (
	cameraRegistry   = map[string]CreateCamera{}
	armRegistry      = map[string]CreateArm{}
	gripperRegistry  = map[string]CreateGripper{}
	providerRegistry = map[string]CreateProvider{}
	baseRegistry     = map[string]CreateBase{}
)

func RegisterCamera(model string, f CreateCamera) {
	_, old := cameraRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two cameras with same model %s", model))
	}
	cameraRegistry[model] = f
}

func RegisterArm(model string, f CreateArm) {
	_, old := armRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two arms with same model %s", model))
	}
	armRegistry[model] = f
}

func RegisterGripper(model string, f CreateGripper) {
	_, old := gripperRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two grippers with same model %s", model))
	}
	gripperRegistry[model] = f
}

func RegisterProvider(model string, f CreateProvider) {
	_, old := providerRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two providers with same model %s", model))
	}
	providerRegistry[model] = f
}

func RegisterBase(model string, f CreateBase) {
	_, old := baseRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two bases with same model %s", model))
	}
	baseRegistry[model] = f
}

func CameraLookup(model string) CreateCamera {
	return cameraRegistry[model]
}

func ArmLookup(model string) CreateArm {
	return armRegistry[model]
}

func GripperLookup(model string) CreateGripper {
	return gripperRegistry[model]
}

func ProviderLookup(model string) CreateProvider {
	return providerRegistry[model]
}

func BaseLookup(model string) CreateBase {
	return baseRegistry[model]
}
