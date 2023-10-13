//go:build no_media

// Package builtin implements a navigation service.
package builtin

import (
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
)

type builtIn struct {
	builtInBase
}

// helper for validate when vision / media libraries are present
func (conf *Config) validateObstacleDetectors(path string, deps []string) error {
	for _, obstacleDetectorPair := range conf.ObstacleDetectors {
		return utils.NewConfigValidationError(path, errors.New("obstacle detectors not supported on no_media builds of RDK"))
	}
	return nil
}

type obstaclesTemp struct {
	obstacleDetectorNamePairs []motion.ObstacleDetectorName
}

func (svc *builtIn) reconfigureObstacleDetectors(deps resource.Dependencies, conf resource.Config, svcConfig *Config) (obstaclesTemp, error) {
	var res obstaclesTemp
	for _, pbObstacleDetectorPair := range svcConfig.ObstacleDetectors {
		return res, utils.NewConfigValidationError(path, errors.New("obstacle detectors not supported on no_media builds of RDK"))
	}
	return res, nil
}
