package api

import (
	"fmt"

	"github.com/edaniels/gostream"
)

type CreateCamera func(r Robot, config Component) (gostream.ImageSource, error)

var (
	cameraRegistry = map[string]CreateCamera{}
)

func RegisterCamera(model string, f CreateCamera) {
	_, old := cameraRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two cameras with same model %s", model))
	}
	cameraRegistry[model] = f
}

func CameraLookup(model string) CreateCamera {
	return cameraRegistry[model]
}
