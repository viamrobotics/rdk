package videosource

import (
	"math"
	"strings"
	"time"

	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/driver/availability"
	"github.com/pion/mediadevices/pkg/driver/camera"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

// Below is adapted from github.com/pion/mediadevices.
// It is further adapted from gostream's query.go
// However, this is the minimum code needed for webcam to work, placed in this directory.
// This vastly improves the debugging and feature development experience, by not over-DRY-ing.

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
		labels := strings.Split(d.Info().Label, camera.LabelSeparator)
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
	for _, d := range baseDrivers {
		logger.Debugw(d.Info().Label, "priority", float32(d.Info().Priority), "type", d.Info().DeviceType)
	}

	driverProperties := queryDriverProperties(filter, logger)
	if len(driverProperties) == 0 {
		logger.Debugw("found no drivers matching filter")
	} else {
		logger.Debugw("found drivers matching specific filter", "count", len(driverProperties))
	}
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
		return nil, prop.Media{}, errors.New("failed to find the best driver that fits the constraints")
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
				logger.Debugw("error opening driver for querying", "error", err)
				// Skip this driver if we failed to open because we can't get the properties
				continue
			}
			needToClose = append(needToClose, d)
			m[d] = d.Properties()
		} else {
			logger.Debugw("driver not available", "name", d.Info().Name, "label", d.Info().Label, "status", status)
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
