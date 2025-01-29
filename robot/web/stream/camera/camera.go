// Package camera provides utilities for working with camera resources in the context of streaming.
package camera

import (
	"context"
	"image"

	"github.com/pion/mediadevices/pkg/prop"

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

// VideoSourceFromCamera converts a camera resource into a gostream VideoSource.
// This is useful for streaming video from a camera resource.
func VideoSourceFromCamera(ctx context.Context, cam camera.Camera) gostream.VideoSource {
	reader := gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
		img, err := camera.DecodeImageFromCamera(ctx, "", nil, cam)
		if err != nil {
			return nil, func() {}, err
		}
		return img, func() {}, nil
	})

	img, err := camera.DecodeImageFromCamera(ctx, "", nil, cam)
	if err == nil {
		return gostream.NewVideoSource(reader, prop.Video{Width: img.Bounds().Dx(), Height: img.Bounds().Dy()})
	}
	// Okay to return empty prop because processInputFrames will tick and set them
	return gostream.NewVideoSource(reader, prop.Video{})
}
