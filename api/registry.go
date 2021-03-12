package api

import (
	"github.com/edaniels/gostream"
)

type CreateCamera func(r Robot, config Component) (gostream.ImageSource, error)

var (
	cameraRegistry = map[string]CreateCamera{}
)

func RegisterCamera(model string, f CreateCamera) {
	cameraRegistry[model] = f
}

func CameraLookup(model string) CreateCamera {
	return cameraRegistry[model]
}
