// Package camera provides utilities for working with camera resources in the context of streaming.
package camera

import (
	"context"
	"fmt"
	"image"

	"github.com/pion/mediadevices/pkg/prop"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/robot"
)

// Camera returns the camera from the robot (derived from the stream) or
// an error if it has no camera.
func Camera(robot robot.Robot, stream gostream.Stream) (camera.Camera, error) {
	// Stream names are slightly modified versions of the resource short name
	shortName := stream.Name()
	cam, err := camera.FromRobot(robot, shortName)
	if err != nil {
		return nil, err
	}
	return cam, nil
}

// VideoSourceFromCamera converts a camera resource into a gostream VideoSource.
// This is useful for streaming video from a camera resource.
func VideoSourceFromCamera(ctx context.Context, cam camera.Camera) (gostream.VideoSource, error) {
	reader := gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
		namedImages, _, err := cam.Images(ctx, nil, nil)
		if err != nil {
			return nil, func() {}, err
		}
		if len(namedImages) == 0 {
			return nil, func() {}, fmt.Errorf("no images returned from camera")
		}
		img, err := namedImages[0].Image(ctx)
		if err != nil {
			return nil, func() {}, err
		}
		return img, func() {}, nil
	})

	namedImages, _, err := cam.Images(ctx, nil, nil)
	if err != nil {
		// Okay to return empty prop because processInputFrames will tick and set them
		return gostream.NewVideoSource(reader, prop.Video{}), nil //nolint:nilerr
	}
	if len(namedImages) == 0 {
		return gostream.NewVideoSource(reader, prop.Video{}), nil
	}
	img, err := namedImages[0].Image(ctx)
	if err != nil {
		return gostream.NewVideoSource(reader, prop.Video{}), nil //nolint:nilerr
	}

	return gostream.NewVideoSource(reader, prop.Video{Width: img.Bounds().Dx(), Height: img.Bounds().Dy()}), nil
}
