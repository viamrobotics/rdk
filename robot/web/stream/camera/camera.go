// Package camera provides utilities for working with camera resources in the context of streaming.
package camera

import (
	"context"
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
	cam, err := camera.FromProvider(robot, shortName)
	if err != nil {
		return nil, err
	}
	return cam, nil
}

// VideoSourceFromCamera converts a camera resource into a gostream VideoSource.
// This is useful for streaming video from a camera resource.
func VideoSourceFromCamera(ctx context.Context, cam camera.Camera) (gostream.VideoSource, error) {
	reader := gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
		img, err := camera.DecodeImageFromCamera(ctx, "", nil, cam)
		if err != nil {
			return nil, func() {}, err
		}
		return img, func() {}, nil
	})

	// Return empty prop because there are no downstream consumers of the video props anyways.
	// The video encoder's actual properties are set by processInputFrames by sniffing a returned image.
	// We no longer ask the camera for an image in this code path to fill in video props because if the camera
	// hangs on this call, we can potentially block the resource reconfiguration due
	// to the tight coupling of refreshing streams and the resource graph.
	//
	// Blocking the resource reconfiguration is a known issue: https://viam.atlassian.net/browse/RSDK-12744
	//
	// If we ever start relying on the video props for other purposes, we should think of a way to set them
	// without blocking the resource reconfiguration.
	return gostream.NewVideoSource(reader, prop.Video{}), nil
}
