// Package camera provides functions for looking up a camera from a robot using a stream
package camera

import (
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

// Camera returns the camera from the robot (derived from the stream) or
// an error if it has no camera.
func Camera(robot robot.Robot, stream gostream.Stream) (camera.Camera, error) {
	// Stream names are slightly modified versions of the resource short name
	shortName := resource.SDPTrackNameToShortName(stream.Name())
	cam, err := camera.FromRobot(robot, shortName)
	if err != nil {
		return nil, err
	}
	return cam, nil
}
