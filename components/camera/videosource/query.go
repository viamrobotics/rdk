package videosource

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/driver/availability"
	mediadevicescamera "github.com/pion/mediadevices/pkg/driver/camera"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

// Below is adapted from github.com/pion/mediadevices.
// It is further adapted from gostream's query.go
// However, this is the minimum code needed for webcam to work, placed in this directory.
// This vastly improves the debugging and feature development experience, by not over-DRY-ing.

// makeConstraints is a helper that returns constraints to mediadevices in order to find and make a video source.
// Constraints are specifications for the video stream such as frame format, resolution etc.
func makeConstraints(conf *WebcamConfig, logger logging.Logger) mediadevices.MediaStreamConstraints {
	return mediadevices.MediaStreamConstraints{
		Video: func(constraint *mediadevices.MediaTrackConstraints) {
			if conf.Width > 0 {
				constraint.Width = prop.IntExact(conf.Width)
			} else {
				constraint.Width = prop.IntRanged{Min: minResolutionDimension, Ideal: 640, Max: 4096}
			}

			if conf.Height > 0 {
				constraint.Height = prop.IntExact(conf.Height)
			} else {
				constraint.Height = prop.IntRanged{Min: minResolutionDimension, Ideal: 480, Max: 2160}
			}

			if conf.FrameRate > 0.0 {
				constraint.FrameRate = prop.FloatExact(conf.FrameRate)
			} else {
				constraint.FrameRate = prop.FloatRanged{Min: 0.0, Ideal: 30.0, Max: 140.0}
			}

			if conf.Format == "" {
				constraint.FrameFormat = prop.FrameFormatOneOf{
					frame.FormatI420,
					frame.FormatI444,
					frame.FormatYUY2,
					frame.FormatUYVY,
					frame.FormatRGBA,
					frame.FormatMJPEG,
					frame.FormatNV12,
					frame.FormatNV21,
					frame.FormatZ16,
				}
			} else {
				constraint.FrameFormat = prop.FrameFormatExact(conf.Format)
			}

			logger.Debugf("constraints: %v", constraint)
		},
	}
}

// findReaderAndDriver finds a video device and returns an image reader and the driver instance,
// as well as the path to the driver.
func findReaderAndDriver(
	conf *WebcamConfig,
	path string,
	logger logging.Logger,
) (video.Reader, driver.Driver, string, error) {
	mediadevicescamera.Initialize()
	constraints := makeConstraints(conf, logger)

	// Handle specific path
	if path != "" {
		resolvedPath, err := filepath.EvalSymlinks(path)
		if err == nil {
			path = resolvedPath
		}
		reader, driver, err := getReaderAndDriver(filepath.Base(path), constraints, logger)
		if err != nil {
			return nil, nil, "", err
		}

		img, release, err := reader.Read()
		if release != nil {
			defer release()
		}
		if err != nil {
			return nil, nil, "", err
		}

		if conf.Width != 0 && conf.Height != 0 {
			if img.Bounds().Dx() != conf.Width || img.Bounds().Dy() != conf.Height {
				return nil, nil, "", errors.Errorf("requested width and height (%dx%d) are not available for this webcam"+
					" (closest driver found supports resolution %dx%d)",
					conf.Width, conf.Height, img.Bounds().Dx(), img.Bounds().Dy())
			}
		}
		return reader, driver, path, nil
	}

	// Handle "any" path
	reader, driver, err := getReaderAndDriver("", constraints, logger)
	if err != nil {
		return nil, nil, "", errors.Wrap(err, "found no webcams")
	}
	labels := strings.Split(driver.Info().Label, mediadevicescamera.LabelSeparator)
	if len(labels) == 0 {
		logger.Error("no labels parsed from driver")
		return nil, nil, "", nil
	}
	path = labels[0] // path is always the first element

	return reader, driver, path, nil
}

// GetNamedVideoSource attempts to find a device (not a screen) by the given name.
// If name is empty, it finds any device.
func getReaderAndDriver(
	name string,
	constraints mediadevices.MediaStreamConstraints,
	logger logging.Logger,
) (video.Reader, driver.Driver, error) {
	var ptr *string
	if name == "" {
		ptr = nil
	} else {
		ptr = &name
	}
	d, selectedMedia, err := getUserVideoDriver(constraints, ptr, logger)
	if err != nil {
		return nil, nil, err
	}
	reader, err := newReaderFromDriver(d, selectedMedia)
	if err != nil {
		return nil, nil, err
	}
	return reader, d, nil
}

func getUserVideoDriver(
	constraints mediadevices.MediaStreamConstraints,
	label *string,
	logger logging.Logger,
) (driver.Driver, prop.Media, error) {
	var videoConstraints mediadevices.MediaTrackConstraints
	if constraints.Video != nil {
		constraints.Video(&videoConstraints)
	}
	return selectVideo(videoConstraints, label, logger)
}

func newReaderFromDriver(
	videoDriver driver.Driver,
	mediaProp prop.Media,
) (video.Reader, error) {
	recorder, ok := videoDriver.(driver.VideoRecorder)
	if !ok {
		return nil, errors.New("driver not a driver.VideoRecorder")
	}

	if ok, err := driver.IsAvailable(videoDriver); !errors.Is(err, availability.ErrUnimplemented) && !ok {
		return nil, errors.Wrap(err, "video driver not available")
	} else if driverStatus := videoDriver.Status(); driverStatus != driver.StateClosed {
		return nil, errors.New("video driver in use")
	} else if err := videoDriver.Open(); err != nil {
		return nil, errors.Wrap(err, "cannot open video driver")
	}

	mediaProp.DiscardFramesOlderThan = time.Second
	reader, err := recorder.VideoRecord(mediaProp)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

func labelFilter(target string, useSep bool) driver.FilterFn {
	return driver.FilterFn(func(d driver.Driver) bool {
		if !useSep {
			return d.Info().Label == target
		}
		labels := strings.Split(d.Info().Label, mediadevicescamera.LabelSeparator)
		for _, label := range labels {
			if label == target {
				return true
			}
		}
		return false
	})
}

func selectVideo(
	constraints mediadevices.MediaTrackConstraints,
	label *string,
	logger logging.Logger,
) (driver.Driver, prop.Media, error) {
	return selectBestDriver(getVideoFilterBase(), getVideoFilter(label), constraints, logger)
}

func getVideoFilterBase() driver.FilterFn {
	typeFilter := driver.FilterVideoRecorder()
	notScreenFilter := driver.FilterNot(driver.FilterDeviceType(driver.Screen))
	return driver.FilterAnd(typeFilter, notScreenFilter)
}

func getVideoFilter(label *string) driver.FilterFn {
	filter := getVideoFilterBase()
	if label != nil {
		filter = driver.FilterAnd(filter, labelFilter(*label, true))
	}
	return filter
}

// select implements SelectSettings algorithm.
// Reference: https://w3c.github.io/mediacapture-main/#dfn-selectsettings
func selectBestDriver(
	baseFilter driver.FilterFn,
	filter driver.FilterFn,
	constraints mediadevices.MediaTrackConstraints,
	logger logging.Logger,
) (driver.Driver, prop.Media, error) {
	var bestDriver driver.Driver
	var bestProp prop.Media
	minFitnessDist := math.Inf(1)

	baseDrivers := driver.GetManager().Query(baseFilter)
	logger.Debugw("before specific filter, we found the following drivers", "count", len(baseDrivers))
	for i, d := range baseDrivers {
		props := d.Properties()
		logger.Debugw("base driver found",
			"driver_number", fmt.Sprintf("%d/%d", i+1, len(baseDrivers)),
			"label", d.Info().Label,
			"priority", float32(d.Info().Priority),
			"type", d.Info().DeviceType,
			"properties", props)
	}

	driverProperties := queryDriverProperties(filter, logger)
	if len(driverProperties) == 0 {
		return nil, prop.Media{}, errors.New("found no queryable drivers matching filter")
	}

	logger.Debugw("found drivers matching specific filter", "count", len(driverProperties))
	for d, props := range driverProperties {
		priority := float64(d.Info().Priority)
		logger.Debugw(
			"considering driver",
			"label", d.Info().Label,
			"priority", priority)
		for _, p := range props {
			fitnessDist, ok := constraints.MediaConstraints.FitnessDistance(p)
			if !ok {
				logger.Debugw("driver does not satisfy any constraints", "label", d.Info().Label)
				continue
			}
			fitnessDistWithPriority := fitnessDist - priority
			logger.Debugw(
				"driver properties satisfy some constraints",
				"label", d.Info().Label,
				"props", p,
				"distance", fitnessDist,
				"distance_with_priority", fitnessDistWithPriority)
			if fitnessDistWithPriority < minFitnessDist {
				minFitnessDist = fitnessDistWithPriority
				bestDriver = d
				bestProp = p
			}
		}
	}

	if bestDriver == nil {
		labels := make([]string, 0, len(driverProperties))
		for d := range driverProperties {
			labels = append(labels, d.Info().Label)
		}
		return nil, prop.Media{}, errors.Errorf(
			"failed to find a queryable driver that matches the config constraints. Devices tried: %s",
			strings.Join(labels, ", "))
	}

	logger.Debugw("winning driver", "label", bestDriver.Info().Label, "props", bestProp)
	selectedMedia := prop.Media{}
	selectedMedia.MergeConstraints(constraints.MediaConstraints)
	selectedMedia.Merge(bestProp)
	return bestDriver, selectedMedia, nil
}

func queryDriverProperties(
	filter driver.FilterFn,
	logger logging.Logger,
) map[driver.Driver][]prop.Media {
	var needToClose []driver.Driver
	drivers := driver.GetManager().Query(filter)
	m := make(map[driver.Driver][]prop.Media)

	for _, d := range drivers {
		var status string
		isAvailable, err := driver.IsAvailable(d)
		if errors.Is(err, availability.ErrUnimplemented) {
			s := d.Status()
			status = string(s)
			isAvailable = s == driver.StateClosed
		} else if err != nil {
			status = err.Error()
		}

		if isAvailable {
			err := d.Open()
			if err != nil {
				logger.Infow("error trying to open driver for querying", "error", err)
				// Skip this driver if we failed to open because we can't get the properties
				continue
			}
			needToClose = append(needToClose, d)
			m[d] = d.Properties()
		} else {
			logger.Infow("driver not available", "name", d.Info().Name, "label", d.Info().Label, "status", status)
		}
	}

	for _, d := range needToClose {
		// Since it was closed, we should close it to avoid a leak
		if err := d.Close(); err != nil {
			logger.Errorw("error closing driver", "error", err)
		}
	}

	return m
}
