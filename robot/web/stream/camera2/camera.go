// Package camera provides utilities for working with camera resources in the context of streaming.
package camera

import (
	"context"
	"fmt"
	"image"

	"github.com/pion/mediadevices/pkg/prop"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
	rutils "go.viam.com/rdk/utils"
)

var streamableImageMIMETypes = map[string]interface{}{
	rutils.MimeTypeRawRGBA:  nil,
	rutils.MimeTypeRawDepth: nil,
	rutils.MimeTypeJPEG:     nil,
	rutils.MimeTypePNG:      nil,
	rutils.MimeTypeQOI:      nil,
}

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

// GetStreamableNamedImageFromCamera returns the first named image it finds from the camera that is supported for streaming.
func GetStreamableNamedImageFromCamera(ctx context.Context, cam camera.Camera) (camera.NamedImage, error) {
	namedImages, _, err := cam.Images(ctx, nil, nil)
	if err != nil {
		return camera.NamedImage{}, err
	}
	if len(namedImages) == 0 {
		return camera.NamedImage{}, fmt.Errorf("no images received for camera %q", cam.Name())
	}

	for _, namedImage := range namedImages {
		if _, ok := streamableImageMIMETypes[namedImage.MimeType()]; ok {
			return namedImage, nil
		}
	}
	return camera.NamedImage{}, fmt.Errorf("no images were found with a streamable mime type for camera %q", cam.Name())
}

// getImageBySourceName retrieves a specific named image from the camera by source name.
func getImageBySourceName(ctx context.Context, cam camera.Camera, sourceName string) (camera.NamedImage, error) {
	sourceNames := []string{sourceName}
	namedImages, _, err := cam.Images(ctx, sourceNames, nil)
	if err != nil {
		return camera.NamedImage{}, err
	}
	if len(namedImages) == 0 {
		return camera.NamedImage{}, fmt.Errorf("no images found for requested source name: %s", sourceName)
	}

	// We could have multiple images returned if the camera is on an older version of the API and does not support
	// filtering by source name. In this case, we'll use the first image and tank the performance costs.
	if namedImages[0].SourceName != sourceName {
		return camera.NamedImage{}, fmt.Errorf("mismatched source name: requested %q, got %q", sourceName, namedImages[0].SourceName)
	}

	return namedImages[0], nil
}

// VideoSourceFromCamera converts a camera resource into a gostream VideoSource.
// This is useful for streaming video from a camera resource.
func VideoSourceFromCamera(ctx context.Context, cam camera.Camera) (gostream.VideoSource, error) {
	var sourceName *string
	reader := gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
		var respNamedImage camera.NamedImage

		if sourceName == nil {
			namedImage, err := GetStreamableNamedImageFromCamera(ctx, cam)
			if err != nil {
				return nil, func() {}, err
			}
			respNamedImage = namedImage
			sourceName = &namedImage.SourceName
		} else {
			var err error
			respNamedImage, err = getImageBySourceName(ctx, cam, *sourceName)
			if err != nil {
				sourceName = nil
				return nil, func() {}, err
			}
		}

		img, err := respNamedImage.Image(ctx)
		if err != nil {
			sourceName = nil
			return nil, func() {}, err
		}
		return img, func() {}, nil
	})

	img, _, err := reader(ctx)
	if err != nil {
		// Okay to return empty prop because processInputFrames will tick and set them
		return gostream.NewVideoSource(reader, prop.Video{}), nil //nolint:nilerr
	}
	if lazyImg, ok := img.(*rimage.LazyEncodedImage); ok {
		if err := lazyImg.DecodeConfig(); err != nil {
			return nil, fmt.Errorf("failed to decode lazy encoded image: %w", err)
		}
	}

	return gostream.NewVideoSource(reader, prop.Video{Width: img.Bounds().Dx(), Height: img.Bounds().Dy()}), nil
}
