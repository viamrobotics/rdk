//go:build !no_media

// Package builtin implements a navigation service.
package builtin

import (
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/vision"
)

// ObstacleDetector pairs a vision service with a camera, informing the service about which camera it may use.
type ObstacleDetector struct {
	VisionService vision.Service
	Camera        camera.Camera
}

type builtIn struct {
	resource.Named
	logger golog.Logger
	builtInBase
	obstacleDetectors []*ObstacleDetector
}

// helper for validate when vision / media libraries are present
func (conf *Config) validateObstacleDetectors(path string, deps []string) error {
	for _, obstacleDetectorPair := range conf.ObstacleDetectors {
		if obstacleDetectorPair.VisionServiceName == "" || obstacleDetectorPair.CameraName == "" {
			return utils.NewConfigValidationError(path, errors.New("an obstacle detector is missing either a camera or vision service"))
		}
		deps = append(deps, resource.NewName(vision.API, obstacleDetectorPair.VisionServiceName).String())
		deps = append(deps, resource.NewName(camera.API, obstacleDetectorPair.CameraName).String())
	}
	return nil
}

// struct that holds obstacle detector config on normal builds, degrades gracefully on no_media builds
type obstaclesTemp struct {
	obstacleDetectorNamePairs []motion.ObstacleDetectorName
	obstacleDetectors         []*ObstacleDetector
}

func (svc *builtIn) setObstacles(obstacles obstaclesTemp) {
	svc.obstacleDetectors = obstacles.obstacleDetectors
}

func (svc *builtIn) numObstacleDetectors() int {
	return len(svc.obstacleDetectors)
}

func (svc *builtIn) reconfigureObstacleDetectors(deps resource.Dependencies, conf resource.Config, svcConfig *Config) (obstaclesTemp, error) {
	var res obstaclesTemp
	for _, pbObstacleDetectorPair := range svcConfig.ObstacleDetectors {
		visionSvc, err := vision.FromDependencies(deps, pbObstacleDetectorPair.VisionServiceName)
		if err != nil {
			return res, err
		}
		camera, err := camera.FromDependencies(deps, pbObstacleDetectorPair.CameraName)
		if err != nil {
			return res, err
		}
		res.obstacleDetectorNamePairs = append(res.obstacleDetectorNamePairs, motion.ObstacleDetectorName{
			VisionServiceName: visionSvc.Name(), CameraName: camera.Name(),
		})
		res.obstacleDetectors = append(res.obstacleDetectors, &ObstacleDetector{
			VisionService: visionSvc, Camera: camera,
		})
	}
	return res, nil
}
