//go:build no_media

// Package builtin implements a navigation service.
package builtin

import (
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
)

type builtIn struct {
	resource.Named
	logger golog.Logger
	builtInBase
}

// helper for validate when vision / media libraries are present
func (conf *Config) validateObstacleDetectors(path string, deps []string) ([]string, error) {
	for range conf.ObstacleDetectors {
		return deps, utils.NewConfigValidationError(path, errors.New("obstacle detectors not supported on no_media builds of RDK"))
	}
	return deps, nil
}

type obstaclesTemp struct {
	obstacleDetectorNamePairs []motion.ObstacleDetectorName
}

// stub version of this for when camera is not available
func (svc *builtIn) setObstacles(obstacles obstaclesTemp) {}

// stub version of this for when camera is not available
func (svc *builtIn) numObstacleDetectors() int {
	return 0
}

func (svc *builtIn) reconfigureObstacleDetectors(deps resource.Dependencies, conf resource.Config, svcConfig *Config) (obstaclesTemp, error) {
	var res obstaclesTemp
	for range svcConfig.ObstacleDetectors {
		return res, errors.New("obstacle detectors not supported on no_media builds of RDK")
	}
	return res, nil
}
